// SPDX-FileCopyrightText: 2022 UnionTech Software Technology Co., Ltd.
//
// SPDX-License-Identifier: GPL-3.0-or-later

#ifndef __TOUCHSCREEN_CORE_H__
#define __TOUCHSCREEN_CORE_H__

#include <libinput.h>
#include <stdbool.h>

#define MOV_SLOTS 10  // number of slots (eg maximum number of supported touch points


enum Direction {            // if applied to edges, will just denote the side of the edge
	DIR_NONE,
	DIR_TOP,          // movement towards top
	DIR_RIGHT,        // movement towards right
	DIR_BOT,          // movement towards bottom
	DIR_LEFT,         // movement towards left
};
typedef enum Direction Direction;

enum GestureType {
	GT_NONE,          // not a gesture
	GT_TAP,           // single tap on the screen with no movement
	GT_MOVEMENT,      // general moving gesture
	GT_EDGE,        // movement starting on edge of screen
};

typedef struct gesture {
	enum GestureType type;     // type of gesture
	enum Direction dir;     // direction
	int num;                // number of fingers
} gesture;

typedef struct point {
	double x;
	double y;
} point;

typedef struct movement {
	point start;
	uint32_t t_start;
	point end;
	uint32_t t_end;
	bool ready;
	bool down;
} movement;

typedef struct screenInfo {   //touchscreen info
    uint32_t width;
    uint32_t height;
} screeninfo;

static struct moveStop {         //calculate  move stop time
    uint32_t start;
    double x, y;
} moveStop;

static point last_point;
static point start_point_scale;
static point last_point_scale;
static screeninfo screen;
static int move_stop_distance = 1;
static int edge_error_limit = 3;	//edge error limit when swipe to touchscreen from edge
static uint32_t edge_move_stop_time = 0;
static Direction edge_move_stop_direction = DIR_NONE;
static double min_edge_distance = 10.0;   // minimum gesture distance from edge (in mm)

int get_edge_type();
int get_movement_type();
point get_last_point_scale();
void set_edge_move_stop_time(int duration);
void handle_movements(movement *m);
void get_screen_info(struct libinput_event *event);
void handle_touch_event_down(struct libinput_event *event, struct movement *m);
void handle_touch_event_up(struct libinput_event *event, struct movement *m);
void handle_touch_event_cancel(struct libinput_event *event, struct movement *m);
void handle_touch_event_motion(struct libinput_event *event, struct movement *m);


#endif
