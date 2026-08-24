#ifndef BYEDPI_SHIM_H
#define BYEDPI_SHIM_H

/* Embedded ciadpi (ByeDPI) control surface for libbox.
 * Mirrors app/src/main/cpp/native-lib.c of the Android SFA-ByeDPI fork. */

int  bd_start(int argc, char **argv); /* blocking: runs the proxy event loop */
void bd_stop(void);                   /* graceful: shutdown(server_fd) */
void bd_force_close(void);            /* hard: close(server_fd) */
int  bd_running(void);

#endif
