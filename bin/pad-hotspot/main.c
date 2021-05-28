#define _GNU_SOURCE
#include <stdio.h>
#include <stdlib.h>
#include <assert.h>
#include <string.h>
#include <errno.h>
#include <stdbool.h>
#include <fcntl.h>
#include <unistd.h>

#define SOFTAP_VERSION "1.0"
#define DBG true

#if DBG
#define DEBUG_INFO(M, ...) printf("DEBUG %d: " M, __LINE__, ##__VA_ARGS__)
#else
#define DEBUG_INFO(M, ...) do {} while (0)
#endif
#define DEBUG_ERR(M, ...) printf("DEBUG %d: " M, __LINE__, ##__VA_ARGS__)

//rtl8723bs and ap6214
static char wifi_type[64];

#define WIFI_CHIP_TYPE_PATH     "/sys/class/rkwifi/chip"
static const char SOFTAP_INTERFACE_STATIC_IP[] = "192.168.88.1";
static const char DNSMASQ_CONF_DIR[] = "/etc/dnsmasq.conf";
// hostapd
static const char HOSTAPD_CONF_DIR[] = "/usr/share/doc/hostapd/examples/hostapd.conf";
static char softap_name[64];

static const bool console_run(const char *cmdline)
{
    DEBUG_INFO("cmdline = %s\n", cmdline);
    int ret;
    ret = system(cmdline);
    if (ret < 0) {
        DEBUG_ERR("Running cmdline failed: %s\n", cmdline);
        return false;
    }
    return true;
}


static int get_pid(char *Name)
{
    int len;
    char name[20] = {0};
    len = strlen(Name);
    strncpy(name,Name,len);
    name[len] ='\0';
    char cmdresult[256] = {0};
    char cmd[20] = {0};
    FILE *pFile = NULL;
    int  pid = 0;

    sprintf(cmd, "pidof %s", name);
    pFile = popen(cmd, "r");
    if (pFile != NULL) {
        while (fgets(cmdresult, sizeof(cmdresult), pFile)) {
            pid = atoi(cmdresult);
            DEBUG_INFO("--- %s pid = %d ---\n", name, pid);
            break;
        }
    }
    pclose(pFile);
    return pid;
}

static int get_dnsmasq_pid()
{
    int ret;
    ret = get_pid("dnsmasq");
    return ret;
}

static int get_hostapd_pid()
{
    int ret;
    ret = get_pid("hostapd");
    return ret;
}


static int wifi_rtl_stop_hostapd()
{
    int pid;
    char *cmd = NULL;

    pid = get_hostapd_pid();

    if (pid!=0) {
        asprintf(&cmd, "kill %d", pid);
        console_run(cmd);
        console_run("killall hostapd");
        free(cmd);
    }
    return 0;
}

static int create_hostapd_file(const char* name, const char* password)
{
    FILE* fp;
    char cmdline[256] = {0};

    fp = fopen(HOSTAPD_CONF_DIR, "wt+");

    if (fp != 0) {
        sprintf(cmdline, "interface=%s\n", softap_name);
        fputs(cmdline, fp);
        fputs("ctrl_interface=/var/run/hostapd\n", fp);
        fputs("driver=nl80211\n", fp);
        fputs("ssid=", fp);
        fputs(name, fp);
        fputs("\n", fp);
        fputs("channel=6\n", fp);
        fputs("hw_mode=g\n", fp);
        fputs("ieee80211n=1\n", fp);
        fputs("ignore_broadcast_ssid=0\n", fp);
#if 0
        fputs("auth_algs=1\n", fp);
        fputs("wpa=3\n", fp);
        fputs("wpa_passphrase=", fp);
        fputs(password, fp);
        fputs("\n", fp);
        fputs("wpa_key_mgmt=WPA-PSK\n", fp);
        fputs("wpa_pairwise=TKIP\n", fp);
        fputs("rsn_pairwise=CCMP", fp);
#endif
        fclose(fp);
        return 0;
    }
    return -1;
}

static bool creat_dnsmasq_file()
{
    FILE* fp;
    fp = fopen(DNSMASQ_CONF_DIR, "wt+");
    if (fp != 0) {
        fputs("user=root\n", fp);
        fputs("listen-address=", fp);
        fputs(SOFTAP_INTERFACE_STATIC_IP, fp);
        fputs("\n", fp);
        fputs("dhcp-range=192.168.88.50,192.168.88.150\n", fp);
        fputs("server=/google/8.8.8.8\n", fp);
        fclose(fp);
        return true;
    }
    DEBUG_ERR("---open dnsmasq configuarion file failed!!---");
    return true;
}

static int wlan_accesspoint_start(const char* ssid, const char* password)
{
    char cmdline[256] = {0};
    create_hostapd_file(ssid, password);

    console_run("killall dnsmasq");
    sprintf(cmdline, "ifconfig %s up", softap_name);
    console_run(cmdline);
    sprintf(cmdline, "ifconfig %s 192.168.88.1 netmask 255.255.255.0", softap_name);
    console_run(cmdline);
    //sprintf(cmdline, "route add default gw 192.168.88.1 %s", softap_name);
    //console_run(cmdline);
    creat_dnsmasq_file();
    int dnsmasq_pid = get_dnsmasq_pid();
    if (dnsmasq_pid != 0) {
        memset(cmdline, 0, sizeof(cmdline));
        sprintf(cmdline, "kill %d", dnsmasq_pid);
        console_run(cmdline);
    }
    memset(cmdline, 0, sizeof(cmdline));
    sprintf(cmdline, "dnsmasq -C %s --interface=%s", DNSMASQ_CONF_DIR, softap_name);
    console_run(cmdline);

    memset(cmdline, 0, sizeof(cmdline));
    sprintf(cmdline, "hostapd %s &", HOSTAPD_CONF_DIR);
    console_run(cmdline);
    return 1;
}

static const int wifi_dhd_start_softap(const char* apName, const char* apPassword)
{
    char cmdline[256] = {0};
    // 1. enable WiFi interface
    console_run("ifconfig wlan0 up");
    // 2. initiate apsta mode
    console_run("dhd_priv iapsta_init mode apsta");
    // 3. eable softap mode with ssid name
    memset(cmdline, 0, sizeof(cmdline));
    sprintf(cmdline, "dhd_priv iapsta_config ifname wlan0 ssid %s chan 6 amode open emode none", apName);
    console_run(cmdline);
    console_run("dhd_priv iapsta_enable ifname wlan0");
    // 4. set static IP addres
    memset(cmdline, 0, sizeof(cmdline));
    sprintf(cmdline, "ifconfig wlan0 %s", SOFTAP_INTERFACE_STATIC_IP);
    console_run(cmdline);
    // 5. dnsmasql wl0.1 interface
    // creat_dnsmasq_file();
    int dnsmasq_pid = get_dnsmasq_pid();
    if (dnsmasq_pid != 0) {
        memset(cmdline, 0, sizeof(cmdline));
        sprintf(cmdline, "kill %d", dnsmasq_pid);
        console_run(cmdline);
    }
    memset(cmdline, 0, sizeof(cmdline));
    sprintf(cmdline, "dnsmasq -C %s --interface=wlan0", DNSMASQ_CONF_DIR);
    console_run(cmdline);
    return 1;
}

static const int wifi_dhd_stop_softap()
{
    console_run("dhd_priv iapsta_disable ifname wlan0");
    return 0;
}

static const int iftables_eth0_to_wl0()
{
    char cmdline[256] = {0};

    console_run("ifconfig wlan0 up");
    memset(cmdline, 0, sizeof(cmdline));
    sprintf(cmdline, "ifconfig wlan0 %s netmask 255.255.255.0", SOFTAP_INTERFACE_STATIC_IP);
    console_run(cmdline);
    console_run("echo 1 > /proc/sys/net/ipv4/ip_forward");
    console_run("iptables --flush");
    console_run("iptables --table nat --flush");
    console_run("iptables --delete-chain");
    console_run("iptables --table nat --delete-chain");
    console_run("iptables --table nat --append POSTROUTING --out-interface wlan0 -j MASQUERADE");
    console_run("iptables --append FORWARD --in-interface wlan0 -j ACCEPT");
    return 0;
}

static const int iftables_eth0_to_p2p0()
{
    char cmdline[256] = {0};

    console_run("ifconfig wlan0 up");
    memset(cmdline, 0, sizeof(cmdline));
    sprintf(cmdline, "ifconfig p2p0 %s netmask 255.255.255.0", SOFTAP_INTERFACE_STATIC_IP);
    console_run(cmdline);
    console_run("echo 1 > /proc/sys/net/ipv4/ip_forward");
    console_run("iptables --flush");
    console_run("iptables --table nat --flush");
    console_run("iptables --delete-chain");
    console_run("iptables --table nat --delete-chain");
    console_run("iptables --table nat --append POSTROUTING --out-interface p2p0 -j MASQUERADE");
    console_run("iptables --append FORWARD --in-interface wlan0 -j ACCEPT");
    return 0;
}

static int check_wifi_chip_type_string(char *type)
{
    int wififd, ret = 0;
    char buf[64];

    wififd = open(WIFI_CHIP_TYPE_PATH, O_RDONLY);
    if (wififd < 0 ) {
        DEBUG_ERR("Can't open %s, errno = %d", WIFI_CHIP_TYPE_PATH, errno);
        ret = -1;
        goto fail_exit;
    }
    memset(buf, 0, 64);

    if (0 == read(wififd, buf, 32)) {
        DEBUG_ERR("read %s failed", WIFI_CHIP_TYPE_PATH);
        close(wififd);
        ret = -1;
        goto fail_exit;
    }
    close(wififd);

    strcpy(type, buf);
    DEBUG_INFO("%s: %s", __FUNCTION__, type);

fail_exit:
    return ret;
}

int main(int argc, char **argv)
{
    char *str_stop = "stop";
    const char *apName = "RK_SOFTAP_TEST";

    DEBUG_INFO("\nsoftap_version: %s\n", SOFTAP_VERSION);

    check_wifi_chip_type_string(wifi_type);
    DEBUG_INFO("\nwifi type: %s\n",wifi_type);

    if (!strncmp(wifi_type, "RTL", 3))
        strcpy(softap_name, "p2p0");
    else
        strcpy(softap_name, "wlan0");

    if (argc >= 2) {
        if (!strncmp(wifi_type, "RTL", 3)) {
            if (strcmp(argv[1], str_stop) == 0) {
                DEBUG_INFO("-stop softap-\n");
                wifi_rtl_stop_hostapd();
                system("ifconfig p2p0 down");
                return 0;
            }
        } else {
            if (strcmp(argv[1],str_stop) == 0) {
                DEBUG_INFO("-stop softap-\n");
                wifi_rtl_stop_hostapd();
                console_run("killall dnsmasq");
                console_run("ifconfig wlan0 down");
                return 0;
            }
        }
        apName = argv[1];
    }

    console_run("killall dnsmasq");
    console_run("killall hostapd");
    console_run("killall udhcpc");

    DEBUG_INFO("start softap with name: %s---", apName);
    if (!strncmp(wifi_type, "RTL", 3)) {
        console_run("ifconfig p2p0 down");
        console_run("rm -rf /userdata/bin/p2p0");
        wlan_accesspoint_start(apName,"123456789");
        //iftables_eth0_to_p2p0();
    } else {
        console_run("ifconfig wlan0 down");
        console_run("rm -rf /userdata/bin/wlan0");
        console_run("iw dev wlan0 del");
        console_run("ifconfig wlan1 up");
        if (!strncmp(wifi_type, "AP6181", 6))
            console_run("iw dev wlan0 interface add wlan0 type __ap");
        else
            console_run("iw phy0 interface add wlan0 type managed");
        wlan_accesspoint_start(apName,"123456789");
        //wifi_dhd_start_softap(apName,"123456789");
        //iftables_eth0_to_wl0();
    }
    return 0;
}
