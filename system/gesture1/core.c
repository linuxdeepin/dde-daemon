// SPDX-FileCopyrightText: 2018 - 2026 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#include <stdio.h>
#include <stdint.h>
#include <math.h>
#include <errno.h>
#include <syslog.h>
#include <libinput.h>
#include <string.h>
#include <unistd.h>
#include <fcntl.h>

#include <glib.h>
#include <poll.h>

#include "utils.h"
#include "core.h"
#include "_cgo_export.h"

#define ALARM_TIMEOUT_DEFAULT 700 // 700ms
#define LONG_PRESS_MAX_DISTANCE 3
#define SCREEN_WIDTH 100
#define SCREEN_HEIGHT 100

struct raw_multitouch_event {
    double dx_unaccel, dy_unaccel;
    double scale;
    int fingers;
    uint64_t t_start_tap;
    guint tap_id;
    guint tap_generation;
    char *device_node;
    bool dblclick;
    bool ignore;
};

struct tap_timer_data {
    char *device_node;
    guint generation;
    int fingers;
};

static void raw_event_reset(struct raw_multitouch_event *event, bool reset_dblclick);
static struct raw_multitouch_event *raw_event_new(const char *device_node);
static void raw_event_free(gpointer data);
static void handle_tap_stop(struct raw_multitouch_event *event);

static int is_touchpad(struct libinput_device *dev);
static const char* get_multitouch_device_node(struct libinput_event *ev);

static void handle_events(struct libinput *li, struct movement *m);
static void handle_gesture_events(struct libinput_event *ev, int type);

static gboolean touch_timer_handler(gpointer data);
static void touch_timer_destroy(gpointer data);
static void start_touch_timer();
static void stop_touch_timer();
static void release_touch_right_button(void);
static int touch_right_button_sent(void);
static void stop_all_active_swipes(void);
static int valid_long_press_touch(double x, double y);

static GHashTable *ev_table = NULL;
struct raw_multitouch_event *raw = NULL;
static GMutex tap_mutex;
static GMutex right_button_mutex;

static GMutex timer_callback_mutex;
static GCond timer_callback_cond;
static guint active_timer_callbacks = 0;
static bool timer_callbacks_stopping = true;

static GMutex loop_mutex;
static GCond loop_start_cond;
static int quit_requested = 0;
static int quit_write_fd = -1;
static bool loop_active = false;
static bool loop_start_reported = false;
static int loop_start_result = 0;

static struct _long_press_timer {
    guint id;
    guint fingers;
    gint sent; // mousedown event sent
    double x, y;
    uint32_t width;
    uint32_t height;
} touch_timer;

static struct _short_press_timer {
    guint id;
} short_press_timer;

guint long_press_timer2 = 0;

static int long_press_duration = ALARM_TIMEOUT_DEFAULT;
static int long_press_duration2 = 1000;
static double long_press_distance = LONG_PRESS_MAX_DISTANCE;
static int short_press_duration = 200;
static int dblclick_duration = 0; // 判断两次单击的间隔，是否为双击

static uint64_t _prev_ev_time; // 前一个非 TOUCH_FRAME 事件的时间，单位 usec
static int _prev_ev_type; // 前一个非 TOUCH_FRAME 事件的类型
static uint64_t _prev_frame_ev_time; // 前一个 TOUCH_FRAME 事件的时间，单位 usec

static bool
timer_callback_enter(void)
{
    bool allowed = false;

    g_mutex_lock(&timer_callback_mutex);
    if (!timer_callbacks_stopping) {
        active_timer_callbacks++;
        allowed = true;
    }
    g_mutex_unlock(&timer_callback_mutex);
    return allowed;
}

static void
timer_callback_leave(void)
{
    g_mutex_lock(&timer_callback_mutex);
    active_timer_callbacks--;
    if (timer_callbacks_stopping && active_timer_callbacks == 0) {
        g_cond_signal(&timer_callback_cond);
    }
    g_mutex_unlock(&timer_callback_mutex);
}

static void
start_timer_callbacks(void)
{
    g_mutex_lock(&timer_callback_mutex);
    timer_callbacks_stopping = false;
    g_mutex_unlock(&timer_callback_mutex);
}

static void
stop_timer_callbacks(void)
{
    g_mutex_lock(&timer_callback_mutex);
    timer_callbacks_stopping = true;
    while (active_timer_callbacks != 0) {
        g_cond_wait(&timer_callback_cond, &timer_callback_mutex);
    }
    g_mutex_unlock(&timer_callback_mutex);
}

static struct raw_multitouch_event *
raw_event_new(const char *device_node)
{
    struct raw_multitouch_event *event = g_new0(struct raw_multitouch_event, 1);
    event->device_node = g_strdup(device_node);
    return event;
}

static void
raw_event_free(gpointer data)
{
    struct raw_multitouch_event *event = data;
    if (!event) {
        return;
    }
    g_free(event->device_node);
    g_free(event);
}

static void
cancel_all_tap_timers(void)
{
    GArray *tap_ids = g_array_new(FALSE, FALSE, sizeof(guint));

    g_mutex_lock(&tap_mutex);
    if (ev_table) {
        GHashTableIter iter;
        gpointer value;
        g_hash_table_iter_init(&iter, ev_table);
        while (g_hash_table_iter_next(&iter, NULL, &value)) {
            struct raw_multitouch_event *event = value;
            if (event->tap_id) {
                g_array_append_val(tap_ids, event->tap_id);
                event->tap_id = 0;
                event->tap_generation++;
            }
        }
    }
    g_mutex_unlock(&tap_mutex);

    for (guint i = 0; i < tap_ids->len; i++) {
        g_source_remove(g_array_index(tap_ids, guint, i));
    }
    g_array_free(tap_ids, TRUE);
}

static void
reset_loop_state(void)
{
    g_mutex_lock(&loop_mutex);
    quit_write_fd = -1;
    quit_requested = 0;
    loop_active = false;
    g_mutex_unlock(&loop_mutex);
}

static void
report_loop_start(int result)
{
    g_mutex_lock(&loop_mutex);
    loop_start_result = result;
    loop_start_reported = true;
    g_cond_broadcast(&loop_start_cond);
    g_mutex_unlock(&loop_mutex);
}

void
prepare_loop(void)
{
    g_mutex_lock(&loop_mutex);
    quit_requested = 0;
    loop_active = true;
    loop_start_reported = false;
    loop_start_result = 0;
    g_mutex_unlock(&loop_mutex);
}

int
wait_loop_ready(void)
{
    g_mutex_lock(&loop_mutex);
    while (!loop_start_reported) {
        g_cond_wait(&loop_start_cond, &loop_mutex);
    }
    int result = loop_start_result;
    g_mutex_unlock(&loop_mutex);
    return result;
}

int
start_loop(int verbose, double distance)
{
    int wake_pipe[2] = {-1, -1};
    if (pipe(wake_pipe) < 0) {
        fprintf(stderr, "Failed to create gesture wake pipe: %s\n", strerror(errno));
        report_loop_start(0);
        reset_loop_state();
        return -1;
    }
    fcntl(wake_pipe[0], F_SETFL, fcntl(wake_pipe[0], F_GETFL) | O_NONBLOCK);
    fcntl(wake_pipe[1], F_SETFL, fcntl(wake_pipe[1], F_GETFL) | O_NONBLOCK);
    fcntl(wake_pipe[0], F_SETFD, FD_CLOEXEC);
    fcntl(wake_pipe[1], F_SETFD, FD_CLOEXEC);

    g_mutex_lock(&loop_mutex);
    if (quit_requested) {
        quit_requested = 0;
        loop_active = false;
        g_mutex_unlock(&loop_mutex);
        close(wake_pipe[0]);
        close(wake_pipe[1]);
        report_loop_start(0);
        return 0;
    }
    quit_write_fd = wake_pipe[1];
    g_mutex_unlock(&loop_mutex);

    struct libinput *li = open_from_udev("seat0", NULL, verbose);
    if (!li) {
        report_loop_start(0);
        reset_loop_state();
        close(wake_pipe[0]);
        close(wake_pipe[1]);
        return -1;
    }

    if (verbose) {
        g_setenv("G_MESSAGES_DEBUG", "all", TRUE);
    }
    if (distance > 0) {
        long_press_distance = distance;
    }
    ev_table = g_hash_table_new_full(g_str_hash,
                                     g_str_equal,
                                     (GDestroyNotify)g_free,
                                     raw_event_free);
    if (!ev_table) {
        fprintf(stderr, "Failed to initialize event table\n");
        libinput_unref(li);
        report_loop_start(0);
        reset_loop_state();
        close(wake_pipe[0]);
        close(wake_pipe[1]);
        return -1;
    }
    start_timer_callbacks();
    report_loop_start(1);

    // movements have pointer structs inside
    struct movement movements[MOV_SLOTS] = {{{0}}};

    // firstly handle all devices
    handle_events(li, movements);

    struct pollfd fds[2] = {
        {.fd = libinput_get_fd(li), .events = POLLIN, .revents = 0},
        {.fd = wake_pipe[0], .events = POLLIN, .revents = 0},
    };

    while (true) {
        if (poll(fds, 2, -1) < 0) {
            if (errno == EINTR) {
                continue;
            }
            fprintf(stderr, "Gesture event poll failed: %s\n", strerror(errno));
            break;
        }

        if (fds[1].revents & (POLLIN | POLLERR | POLLHUP)) {
            char buffer[16];
            while (read(wake_pipe[0], buffer, sizeof(buffer)) > 0) {
            }
            break;
        }

        if (fds[0].revents & POLLIN) {
            handle_events(li, movements);
            handle_movements(movements);        //handle touch screen
        }
    }

    stop_timer_callbacks();
    stop_touch_timer();
    release_touch_right_button();
    stop_all_active_swipes();
    cancel_all_tap_timers();

    g_mutex_lock(&tap_mutex);
    raw = NULL;
    g_hash_table_destroy(ev_table);
    ev_table = NULL;
    g_mutex_unlock(&tap_mutex);

    libinput_unref(li);
    reset_loop_state();
    close(wake_pipe[0]);
    close(wake_pipe[1]);
    return 0;
}

void
quit_loop(void)
{
    int wake_fd;

    g_mutex_lock(&loop_mutex);
    if (!loop_active) {
        g_mutex_unlock(&loop_mutex);
        return;
    }
    quit_requested = 1;
    wake_fd = quit_write_fd;
    if (wake_fd >= 0) {
        const char wake = 1;
        ssize_t unused = write(wake_fd, &wake, sizeof(wake));
        (void)unused;
    }
    g_mutex_unlock(&loop_mutex);
}

void
set_timer_duration(int duration)
{
    g_debug("[Duration] set: %d --> %d", long_press_duration, duration);
    if (duration == long_press_duration) {
        return;
    }
    long_press_duration = duration;
}


void
set_timer_short_duration(int duration)
{
    g_debug("[Duration short ] set: %d --> %d", short_press_duration, duration);
    if (duration == short_press_duration) {
        return ;
    }
    short_press_duration = duration;
}

void set_dblclick_duration(int duration) 
{
    if (duration == dblclick_duration) {
        return ;
    }
    dblclick_duration = duration;
}

static void
raw_event_reset(struct raw_multitouch_event *event, bool reset_dblclick)
{
    if (!event) {
        return ;
    }

    event->dx_unaccel = 0.0;
    event->dy_unaccel = 0.0;
    event->scale = 0.0;
    event->fingers = 0;
    event->t_start_tap = 0;
    handle_tap_stop(event);
    if (reset_dblclick)
        event->dblclick = false;
}

static const char*
get_multitouch_device_node(struct libinput_event *ev)
{
    struct libinput_device *dev = libinput_event_get_device(ev);
    if (libinput_device_has_capability(dev, LIBINPUT_DEVICE_CAP_TOUCH)) {
        goto out;
    } else if (libinput_device_has_capability(dev, LIBINPUT_DEVICE_CAP_POINTER) &&
               is_touchpad(dev)) {
        goto out;
    } else {
        return NULL;
    }

out:
    return udev_device_get_devnode(libinput_device_get_udev_device(dev));
}

static int
is_touchpad(struct libinput_device *dev)
{
    // TODO: check touchpad whether support multitouch. fingers > 3?
    int cnt = libinput_device_config_tap_get_finger_count(dev);
    return (cnt > 0);
}

static gboolean
touch_timer_handler(gpointer data)
{
    if (!timer_callback_enter()) {
        return FALSE;
    }
    g_debug("touch timer arrived: %u, data: (%f. %f)", touch_timer.id,
            touch_timer.x, touch_timer.y);
    g_mutex_lock(&right_button_mutex);
    if (!touch_timer.sent) {
        touch_timer.sent = 1;
        handleTouchEvent(TOUCH_TYPE_RIGHT_BUTTON, BUTTON_TYPE_DOWN);
    }
    g_mutex_unlock(&right_button_mutex);
    timer_callback_leave();
    return FALSE;
}

static gboolean
short_press_timer_handler(gpointer data)
{
    if (!timer_callback_enter()) {
        return FALSE;
    }
    point scale = get_last_point_scale();
    handleTouchShortPress(short_press_duration, scale.x, scale.y);
    timer_callback_leave();
    return FALSE;
}

static gboolean
long_press_timer_handler2(gpointer data)
{
    if (!timer_callback_enter()) {
        return FALSE;
    }
    point scale = get_last_point_scale();
    handleTouchPressTimeout(1, long_press_duration2, scale.x, scale.y);
    timer_callback_leave();
    return FALSE;
}

static void
touch_timer_destroy(gpointer data)
{
    g_debug("touch timer destroy: %u, data: (%f, %f)", touch_timer.id,
            touch_timer.x, touch_timer.y);
    if (touch_timer.id != 0) {
        touch_timer.id = 0;
    }
    touch_timer.x = 0;
    touch_timer.y = 0;
}

static void
short_press_timer_destory(gpointer data)
{
    short_press_timer.id = 0;
}

static void
long_press_timer_destroy2(gpointer data)
{
    long_press_timer2 = 0;
}

static void
start_touch_timer()
{
    g_debug("start touch timer: %u, fingers: %d, long_press_duration: %d, short_press_duration: %d", touch_timer.id, touch_timer.fingers, long_press_duration, short_press_duration);
    if (touch_timer.id != 0) {
        g_debug("There has an touch_timer: %u", touch_timer.id);
        return;
    }
    touch_timer.id = g_timeout_add_full(G_PRIORITY_DEFAULT, long_press_duration, touch_timer_handler, NULL, touch_timer_destroy);
    short_press_timer.id = g_timeout_add_full(G_PRIORITY_DEFAULT - 1, short_press_duration, short_press_timer_handler, NULL, short_press_timer_destory);
    long_press_timer2 = g_timeout_add_full(G_PRIORITY_DEFAULT - 2, long_press_duration2, long_press_timer_handler2, NULL, long_press_timer_destroy2);
}

static void
stop_touch_timer()
{
    g_debug("stop touch timer: %u, fingers: %d", touch_timer.id, touch_timer.fingers);

    if (touch_timer.id != 0) {
         g_source_remove(touch_timer.id);
         touch_timer.id = 0;
    }
    if (short_press_timer.id != 0) {
        g_source_remove(short_press_timer.id);
        short_press_timer.id = 0;
    }
    if (long_press_timer2 != 0) {
        g_source_remove(long_press_timer2);
        long_press_timer2 = 0;
    }
}

static void
release_touch_right_button(void)
{
    g_mutex_lock(&right_button_mutex);
    if (touch_timer.sent) {
        touch_timer.sent = 0;
        handleTouchEvent(TOUCH_TYPE_RIGHT_BUTTON, BUTTON_TYPE_UP);
    }
    g_mutex_unlock(&right_button_mutex);
}

static int
touch_right_button_sent(void)
{
    g_mutex_lock(&right_button_mutex);
    int sent = touch_timer.sent;
    g_mutex_unlock(&right_button_mutex);
    return sent;
}

static void
stop_all_active_swipes(void)
{
    GArray *fingers = g_array_new(FALSE, FALSE, sizeof(int));

    g_mutex_lock(&tap_mutex);
    if (ev_table) {
        GHashTableIter iter;
        gpointer value;
        g_hash_table_iter_init(&iter, ev_table);
        while (g_hash_table_iter_next(&iter, NULL, &value)) {
            struct raw_multitouch_event *event = value;
            if (event->dblclick) {
                int finger_count = event->fingers;
                event->dblclick = false;
                g_array_append_val(fingers, finger_count);
            }
        }
    }
    g_mutex_unlock(&tap_mutex);

    for (guint i = 0; i < fingers->len; i++) {
        handleSwipeStop(g_array_index(fingers, int, i));
    }
    g_array_free(fingers, TRUE);
}

static int
valid_long_press_touch(double x, double y)
{
    if (touch_timer.id == 0) {
        return 0;
    }

    double dx = x - touch_timer.x;
    double dy = y - touch_timer.y;
    return fabs(dx) < long_press_distance && fabs(dy) < long_press_distance;
}

static gboolean
handle_tap(gpointer data)
{
    struct tap_timer_data *timer_data = data;
    if (!timer_data || !timer_callback_enter()) {
        return FALSE;
    }

    bool valid = false;
    g_mutex_lock(&tap_mutex);
    if (ev_table) {
        struct raw_multitouch_event *event = g_hash_table_lookup(ev_table, timer_data->device_node);
        if (event && event->tap_id != 0 && event->tap_generation == timer_data->generation) {
            event->tap_id = 0;
            valid = true;
        }
    }
    g_mutex_unlock(&tap_mutex);

    if (valid) {
        g_debug("[Tap] fingers: %d", timer_data->fingers);
        handleGestureEvent(GESTURE_TYPE_TAP, GESTURE_DIRECTION_NONE, timer_data->fingers);
    }
    timer_callback_leave();
    return FALSE;
}

static void
handle_tap_destroy(gpointer data)
{
    struct tap_timer_data *timer_data = data;
    if (!timer_data) {
        return;
    }
    g_free(timer_data->device_node);
    g_free(timer_data);
}

static void
handle_tap_stop(struct raw_multitouch_event *event)
{
    guint tap_id = 0;

    g_mutex_lock(&tap_mutex);
    if (event && event->tap_id) {
        tap_id = event->tap_id;
        event->tap_id = 0;
        event->tap_generation++;
    }
    g_mutex_unlock(&tap_mutex);

    if (tap_id) {
        g_source_remove(tap_id);
    }
}

static int
handle_tap_delay(struct raw_multitouch_event *event)
{
    if (!event || !event->device_node) {
        return 0;
    }

    struct tap_timer_data *timer_data = g_new0(struct tap_timer_data, 1);
    timer_data->device_node = g_strdup(event->device_node);
    timer_data->fingers = event->fingers;

    g_mutex_lock(&tap_mutex);
    if (event->tap_id) {
        g_mutex_unlock(&tap_mutex);
        handle_tap_destroy(timer_data);
        return 0;
    }
    timer_data->generation = ++event->tap_generation;
    event->tap_id = g_timeout_add_full(G_PRIORITY_DEFAULT, dblclick_duration,
                                      handle_tap, timer_data, handle_tap_destroy);
    guint tap_id = event->tap_id;
    g_mutex_unlock(&tap_mutex);

    if (!tap_id) {
        handle_tap_destroy(timer_data);
    }
    return tap_id != 0;
}

/**
 * calculation direction
 * Swipe: (begin -> end)
 *     _dx_unaccel += dx_unaccel, _dy_unaccel += dy_unaccel;
 *     filter small movement threshold abs(_dx_unaccel - _dy_unaccel) < 70
 *     if abs(_dx_unaccel) > abs(_dy_unaccel): _dx_unaccel < 0 ? 'left':'right'
 *     else: _dy_unaccel < 0 ? 'up':'down'
 *
 * Pinch: (begin -> end)
 *     _scale += 1.0 - scale;
 *     if _scale != 0: _scale >= 0 ? 'in':'out'
 **/
static void
handle_gesture_events(struct libinput_event *ev, int type)
{
    struct libinput_device *dev = libinput_event_get_device(ev);
    if (!dev) {
        fprintf(stderr, "Get device from event failure\n");
        return ;
    }

    const char *node = udev_device_get_devnode(libinput_device_get_udev_device(dev));
    g_mutex_lock(&tap_mutex);
    raw = ev_table ? g_hash_table_lookup(ev_table, node) : NULL;
    g_mutex_unlock(&tap_mutex);
    if (!raw) {
        fprintf(stderr, "Not found '%s' in table\n", node);
        return ;
    }
    struct libinput_event_gesture *gesture = libinput_event_get_gesture_event(ev);
    if (raw->dblclick
    && type != LIBINPUT_EVENT_GESTURE_SWIPE_BEGIN
    && type != LIBINPUT_EVENT_GESTURE_SWIPE_UPDATE
    && type != LIBINPUT_EVENT_GESTURE_SWIPE_END
    && type != LIBINPUT_EVENT_GESTURE_HOLD_END) {
        raw->fingers = libinput_event_gesture_get_finger_count(gesture);
        handleSwipeStop(raw->fingers);
        raw->dblclick = false;
    }

    switch (type) {
    case LIBINPUT_EVENT_GESTURE_PINCH_BEGIN:
    case LIBINPUT_EVENT_GESTURE_SWIPE_BEGIN:
        // reset
        raw_event_reset(raw, false);
        raw->fingers = libinput_event_gesture_get_finger_count(gesture);
        break;
    case LIBINPUT_EVENT_GESTURE_PINCH_UPDATE:{
        double scale = libinput_event_gesture_get_scale(gesture);
        raw->scale += 1.0-scale;
        break;
    }
    case LIBINPUT_EVENT_GESTURE_SWIPE_UPDATE:{
        // update
        double dx_unaccel = libinput_event_gesture_get_dx_unaccelerated(gesture);
        double dy_unaccel = libinput_event_gesture_get_dy_unaccelerated(gesture);
        raw->dx_unaccel += dx_unaccel;
        raw->dy_unaccel += dy_unaccel;

        int fingers = libinput_event_gesture_get_finger_count(gesture);
        raw->fingers = fingers;
        if (raw->dblclick) {
            handleSwipeMoving(fingers, dx_unaccel, dy_unaccel);
        }

        break;
    }
    case LIBINPUT_EVENT_GESTURE_PINCH_END:{
        // filter small scale threshold
        if (fabs(raw->scale) < 1) {
            raw_event_reset(raw, true);
            break;
        }

        raw->fingers = libinput_event_gesture_get_finger_count(gesture);
        g_debug("[Pinch] direction: %s, fingers: %d",
                raw->scale>= 0?"in":"out", raw->fingers);
        handleGestureEvent(GESTURE_TYPE_PINCH,
                           (raw->scale >= 0?GESTURE_DIRECTION_IN:GESTURE_DIRECTION_OUT),
                           raw->fingers);
        raw_event_reset(raw, true);
        break;
    }
    case LIBINPUT_EVENT_GESTURE_SWIPE_END:
        if (libinput_event_gesture_get_cancelled(gesture)) {
            raw->fingers = libinput_event_gesture_get_finger_count(gesture);
            if (raw->dblclick) {
                handleSwipeStop(raw->fingers);
            }
            raw_event_reset(raw, true);
            break;
        }

        raw->fingers = libinput_event_gesture_get_finger_count(gesture);
        if (raw->dblclick) {
            handleSwipeStop(raw->fingers);
            raw_event_reset(raw, true);
            break;
        }
        // filter small movement threshold
        if (fabs(raw->dx_unaccel - raw->dy_unaccel) < 70) {
            raw_event_reset(raw, true);
            break;
        }

        if (fabs(raw->dx_unaccel) > fabs(raw->dy_unaccel)) {
            // right/left movement
            g_debug("[Swipe] direction: %s, fingers: %d",
                    raw->dx_unaccel < 0?"left":"right", raw->fingers);
            handleGestureEvent(GESTURE_TYPE_SWIPE,
                               (raw->dx_unaccel < 0?GESTURE_DIRECTION_LEFT:GESTURE_DIRECTION_RIGHT),
                               raw->fingers);
        } else {
            // up/down movement
            g_debug("[Swipe] direction: %s, fingers: %d",
                    raw->dy_unaccel < 0?"up":"down", raw->fingers);
            handleGestureEvent(GESTURE_TYPE_SWIPE,
                               (raw->dy_unaccel < 0?GESTURE_DIRECTION_UP:GESTURE_DIRECTION_DOWN),
                               raw->fingers);
        }

        raw_event_reset(raw, true);
        break;
    case LIBINPUT_EVENT_GESTURE_HOLD_BEGIN:
        g_debug("[Tap begin] time: %u duration: %d fingers: %d \n", raw->t_start_tap, (libinput_event_gesture_get_time_usec(gesture) - raw->t_start_tap) / 1000, raw->fingers);
        if (raw->t_start_tap > 0
        &&  (libinput_event_gesture_get_time_usec(gesture) - raw->t_start_tap) / 1000 <= dblclick_duration
        && raw->fingers == libinput_event_gesture_get_finger_count(gesture)) {
            int fingers = raw->fingers;
            handleDbclickDown(raw->fingers);
            handle_tap_stop(raw);
            raw_event_reset(raw, true);
            raw->fingers = fingers;
            raw->dblclick = true;
        }
        break;
    case LIBINPUT_EVENT_GESTURE_HOLD_END:
        raw->fingers = libinput_event_gesture_get_finger_count(gesture);
        if (raw->dblclick) {
            handleSwipeStop(raw->fingers);
            raw_event_reset(raw, true);
            break;
        }

        if (libinput_event_gesture_get_cancelled(gesture)) {
            raw_event_reset(raw, true);
            break;
        }

        raw->t_start_tap = libinput_event_gesture_get_time_usec(gesture);
        handle_tap_delay(raw);
        break;
    }
}

static void
handle_touch_events(struct libinput_event *ev, int ty,struct movement *m)
{
    point scale;
    struct libinput_device *dev = libinput_event_get_device(ev);
    const char* node= NULL;
    struct raw_multitouch_event *rme = NULL;

    if (!dev) {
        fprintf(stderr, "Get device from event failure\n");
        return ;
    }

    node = get_multitouch_device_node(ev);
    g_mutex_lock(&tap_mutex);
    rme = ev_table ? g_hash_table_lookup(ev_table, node) : NULL;
    g_mutex_unlock(&tap_mutex);
    if (rme && rme->ignore) {
        return;
    }

    struct libinput_event_touch *tevent = libinput_event_get_touch_event(ev);
    uint64_t time_usec = libinput_event_touch_get_time_usec(tevent);
    g_debug("touch event %d time usec: %llu", ty, time_usec);

    // 为修复 pms bug 64939，发现从 libinput 获取到的事件被重复了一遍，比如一次按下，
    // 会出现 TOUCH_DOWN,TOUCH_FRAME,TOUCH_DOWN,TOUCH_FRAME 这样的，于是写了规避的代码。
    // 具体原因可能是 libinput 的 bug，有待调查。
    if (_prev_ev_type == ty && _prev_ev_time == time_usec && _prev_frame_ev_time == time_usec) {
        // 主要特征是时间是相同的
        g_debug("ignore repeat event");
        _prev_ev_type = 0;
        _prev_ev_time = 0;
        _prev_frame_ev_time = 0;
        return;
    }

    if (ty == LIBINPUT_EVENT_TOUCH_FRAME) {
       _prev_frame_ev_time = time_usec;
    } else {
        _prev_ev_time = time_usec;
        _prev_ev_type = ty;
    }

    switch (ty) {
    case LIBINPUT_EVENT_TOUCH_MOTION:{
        handle_touch_event_motion(ev, m);
        g_debug("Touch motion, id: %u, fingers: %d, sent: %d ",
                touch_timer.id, touch_timer.fingers, touch_right_button_sent());
        if (touch_timer.id == 0) {
            break;
        }
        struct libinput_event_touch *touch = libinput_event_get_touch_event(ev);
        // only suupurted events: down and motion
        double x = libinput_event_touch_get_x_transformed(touch, touch_timer.width);
        double y = libinput_event_touch_get_y_transformed(touch, touch_timer.height);
        g_debug("\t[Transformed] X: %lf, Y: %lf", x, y);
        if (valid_long_press_touch(x, y) == 1) {
            break;
        }
        // cancel touch_timer
        stop_touch_timer();
        break;
    }
    case LIBINPUT_EVENT_TOUCH_UP:
        handle_touch_event_up(ev, m);
        g_debug("Touch up, id: %u, fingers: %d, sent: %d ",
                touch_timer.id, touch_timer.fingers, touch_right_button_sent());
        stop_touch_timer();
        if (touch_timer.fingers > 0) {
            touch_timer.fingers--;
        }
        release_touch_right_button();

        scale = get_last_point_scale();
        handleTouchUpOrCancel(scale.x, scale.y);
        return;
    case LIBINPUT_EVENT_TOUCH_DOWN: {
        handle_touch_event_down(ev, m);
        g_debug("Touch down, id: %u, fingers: %d, sent: %d ",
                touch_timer.id, touch_timer.fingers, touch_right_button_sent());
        if (touch_timer.id != 0 || touch_timer.fingers > 0) {
            stop_touch_timer();
            touch_timer.fingers++;
            break;
        }
        struct libinput_event_touch *touch = libinput_event_get_touch_event(ev);
        g_mutex_lock(&right_button_mutex);
        touch_timer.sent = 0;
        g_mutex_unlock(&right_button_mutex);
        start_touch_timer();
        touch_timer.fingers = 1;
        //w和h需要赋初值，防止libinput_device_get_size函数异常返回后，w和h的随机值>1而导致获取的坐标异常
        double w=0.0, h=0.0;
        int iret = libinput_device_get_size(dev, &w, &h);
        // TODO(jouyouyun): save device size to cache
        //libinput_device_get_size会因为abs.is_resolution而直接返回-1
        //如果get size异常，使用SCREEN_WIDTH和SCREEN_HEIGHT来获取坐标
        touch_timer.width = (iret == 0 && w >1) ? w : SCREEN_WIDTH;
        touch_timer.height = (iret == 0 && h >1) ? h : SCREEN_HEIGHT;
        
        touch_timer.x = libinput_event_touch_get_x_transformed(touch, touch_timer.width);
        touch_timer.y = libinput_event_touch_get_y_transformed(touch, touch_timer.height);
        g_debug("\t[Transformed] X: %lf, Y: %lf", touch_timer.x, touch_timer.y);
        break;
    }
    case LIBINPUT_EVENT_TOUCH_FRAME:
        /* g_debug("Touch frame:"); */
        return;
    case LIBINPUT_EVENT_TOUCH_CANCEL:
        handle_touch_event_cancel(ev, m);
        g_debug("Touch cancel, id: %u, fingers: %d, sent: %d ",
                touch_timer.id, touch_timer.fingers, touch_right_button_sent());
        stop_touch_timer();
        touch_timer.fingers = 0;
        release_touch_right_button();

        scale = get_last_point_scale();
        handleTouchUpOrCancel(scale.x, scale.y);
        return;
    }
}

static void
handle_mouse_events(struct libinput_event *ev, int type)
{
    struct libinput_device *dev = libinput_event_get_device(ev);
    if (!dev) {
        fprintf(stderr, "Get device from event failure\n");
        return ;
    }

    struct libinput_event_pointer *mouse = libinput_event_get_pointer_event(ev);
    enum libinput_pointer_axis_source source = libinput_event_pointer_get_axis_source(mouse);
    if (!libinput_event_pointer_has_axis(mouse, LIBINPUT_POINTER_AXIS_SCROLL_HORIZONTAL)) {
        return;
    }
    double value = libinput_event_pointer_get_axis_value(mouse, LIBINPUT_POINTER_AXIS_SCROLL_HORIZONTAL);

    handleMouseEvent(type, source,value);
}

static void
handle_keyboard_events(struct libinput_event *ev, int type)
{
    struct libinput_device *dev = libinput_event_get_device(ev);
    if (!dev) {
        fprintf(stderr, "Get device from event failure\n");
        return ;
    }

    struct libinput_event_keyboard *keyboard = libinput_event_get_keyboard_event(ev);
    uint32_t key = libinput_event_keyboard_get_key(keyboard);
    enum libinput_key_state state = libinput_event_keyboard_get_key_state(keyboard);

    handleKeyboardEvent(key,(uint32_t)state);
}

static void
handle_events(struct libinput *li, struct movement *m)
{
    struct libinput_event *ev;
    libinput_dispatch(li);
    while ((ev = libinput_get_event(li))) {
        int type =libinput_event_get_type(ev);
        switch (type) {
        case LIBINPUT_EVENT_DEVICE_ADDED:{
            const char *path = get_multitouch_device_node(ev);
            if (path) {
                /* g_debug("Device added: %s", path); */
                g_mutex_lock(&tap_mutex);
                if (ev_table && !g_hash_table_contains(ev_table, path)) {
                    g_hash_table_insert(ev_table, g_strdup(path), raw_event_new(path));
                }
                g_mutex_unlock(&tap_mutex);
            }
            break;
        }
        case LIBINPUT_EVENT_DEVICE_REMOVED: {
            const char *path = get_multitouch_device_node(ev);
            if (path) {
                release_touch_right_button();
                /* g_debug("Will remove '%s' to table", path); */
                g_mutex_lock(&tap_mutex);
                struct raw_multitouch_event *event = ev_table ? g_hash_table_lookup(ev_table, path) : NULL;
                g_mutex_unlock(&tap_mutex);
                if (event) {
                    handle_tap_stop(event);
                    if (event->dblclick) {
                        handleSwipeStop(event->fingers);
                    }
                    raw_event_reset(event, true);
                    if (raw == event) {
                        raw = NULL;
                    }
                }
                g_mutex_lock(&tap_mutex);
                if (ev_table) {
                    g_hash_table_remove(ev_table, path);
                }
                g_mutex_unlock(&tap_mutex);
            }
            break;
        }
        case LIBINPUT_EVENT_GESTURE_PINCH_BEGIN:
        case LIBINPUT_EVENT_GESTURE_PINCH_UPDATE:
        case LIBINPUT_EVENT_GESTURE_PINCH_END:
        case LIBINPUT_EVENT_GESTURE_SWIPE_BEGIN:
        case LIBINPUT_EVENT_GESTURE_SWIPE_UPDATE:
        case LIBINPUT_EVENT_GESTURE_SWIPE_END:
        case LIBINPUT_EVENT_GESTURE_HOLD_BEGIN:
        case LIBINPUT_EVENT_GESTURE_HOLD_END:{
            handle_gesture_events(ev, type);
            break;
        }
        case LIBINPUT_EVENT_TOUCH_MOTION:
        case LIBINPUT_EVENT_TOUCH_UP:
        case LIBINPUT_EVENT_TOUCH_DOWN:
        case LIBINPUT_EVENT_TOUCH_CANCEL:
        case LIBINPUT_EVENT_TOUCH_FRAME:{
            handle_touch_events(ev, type, m);
            break;
        }
        case LIBINPUT_EVENT_KEYBOARD_KEY: {
            handle_keyboard_events(ev, type);
            break;
        }
        case LIBINPUT_EVENT_POINTER_AXIS: {
            handle_mouse_events(ev, type);
            break;
        }
        default:
            break;
        }
        libinput_event_destroy(ev);
        libinput_dispatch(li);
    }
}

void set_device_ignore(const char* node, bool ignore) {
    g_mutex_lock(&tap_mutex);
    struct raw_multitouch_event *rme = ev_table ? g_hash_table_lookup(ev_table, node) : NULL;
    if (rme) {
        g_debug("[gesture] set device ignore: %s", ignore ? "true" : "false");
        rme->ignore = ignore;
    }
    g_mutex_unlock(&tap_mutex);
}
