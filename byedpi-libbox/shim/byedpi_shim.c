/* Embedded ciadpi (ByeDPI) shim for libbox.
 * Ported from app/src/main/cpp/native-lib.c of the Android SFA-ByeDPI fork
 * so the proxy can be started repeatedly inside one process lifetime. */

#include <string.h>

#include <arpa/inet.h>
#include <getopt.h>
#include <setjmp.h>
#include <stdlib.h>
#include <sys/socket.h>
#include <unistd.h>

#include "error.h"
#include "params.h"

#include "byedpi_shim.h"

extern int server_fd;

static int g_proxy_running = 0;

struct params default_params = {
        .await_int = 10,
        .ipv6 = 1,
        .resolve = 1,
        .udp = 1,
        .max_open = 512,
        .bfsize = 16384,
        .baddr = {
                .in6 = { .sin6_family = AF_INET6 }
        },
        .laddr = {
                .in = { .sin_family = AF_INET }
        },
        .debug = 0
};

void bd_reset_params(void) {
    clear_params(NULL, NULL);
    params = default_params;
}

int bd_start(int argc, char **argv) {
    if (g_proxy_running) {
        LOG(LOG_S, "byedpi: already running");
        return -1;
    }

    bd_reset_params();
    g_proxy_running = 1;
    optind = 1;
#ifdef __APPLE__
    {
        extern int optreset;
        optreset = 1;
    }
#endif

    int result = ciadpi_main(argc, argv);

    g_proxy_running = 0;
    return result;
}

void bd_stop(void) {
    if (!g_proxy_running) {
        return;
    }
    shutdown(server_fd, SHUT_RDWR);
}

void bd_force_close(void) {
    if (close(server_fd) == -1) {
        return;
    }
    g_proxy_running = 0;
}

int bd_running(void) {
    return g_proxy_running;
}
