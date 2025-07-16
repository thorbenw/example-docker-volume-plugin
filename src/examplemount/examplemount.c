#include <stdio.h>
#include <stdlib.h>
#include <errno.h>
#include <string.h>
#include <sys/inotify.h>
#include <limits.h>
#include <signal.h>
#include <unistd.h>

#define BUF_LEN (10 * (sizeof(struct inotify_event) + NAME_MAX + 1))

int fd = 0;
int wd = 0;

int eprintf(char *message, int safe)
{
    int result = errno;

    if (NULL != message)
    {
        if (safe)
        {
            const int len_max = 256;
            char *message_default = "(message too long)";
            char *message_prefix = "\n";
            char *message_suffix = ": (safe operation doesn't support formatting)\n";

            size_t len_message = strlen(message_prefix) + strlen(message) + strlen(message_suffix);
            if (len_message < len_max)
            {
                message_default = message;
            }
            else
            {
                len_message = strlen(message_prefix) + strlen(message_default) + strlen(message_suffix);
            }

            char msg[len_max]; msg[0] = 0;
            strcat(&msg[0], message_prefix);
            strcat(&msg[0], message_default);
            strcat(&msg[0], message_suffix);
            write(fileno(stderr), msg, len_message);
        }
        else
        {
            fprintf(stderr, "%s: (0x%x) - %s\n", message, result, strerror(result));
        }
    }

    return result;
}

static void displayInotifyEvent(struct inotify_event *i)
{
    printf("Inotify event: wd = 0x%x", i->wd);

    if (i->cookie > 0)
    {
        printf("; cookie = 0x%x", i->cookie);
    }

    printf("; mask = ");
    if (i->mask & IN_ACCESS)        printf("IN_ACCESS ");
    if (i->mask & IN_ATTRIB)        printf("IN_ATTRIB ");
    if (i->mask & IN_CLOSE_NOWRITE) printf("IN_CLOSE_NOWRITE ");
    if (i->mask & IN_CLOSE_WRITE)   printf("IN_CLOSE_WRITE ");
    if (i->mask & IN_CREATE)        printf("IN_CREATE ");
    if (i->mask & IN_DELETE)        printf("IN_DELETE ");
    if (i->mask & IN_DELETE_SELF)   printf("IN_DELETE_SELF ");
    if (i->mask & IN_IGNORED)       printf("IN_IGNORED ");
    if (i->mask & IN_ISDIR)         printf("IN_ISDIR ");
    if (i->mask & IN_MODIFY)        printf("IN_MODIFY ");
    if (i->mask & IN_MOVE_SELF)     printf("IN_MOVE_SELF ");
    if (i->mask & IN_MOVED_FROM)    printf("IN_MOVED_FROM ");
    if (i->mask & IN_MOVED_TO)      printf("IN_MOVED_TO ");
    if (i->mask & IN_OPEN)          printf("IN_OPEN ");
    if (i->mask & IN_Q_OVERFLOW)    printf("IN_Q_OVERFLOW ");
    if (i->mask & IN_UNMOUNT)       printf("IN_UNMOUNT ");

    if (i->len > 0)
    {
        printf("; name = %s", i->name);
    }

    printf("\n");
}

void sighandle(int signo, siginfo_t *info, void *context)
{
    // In case this func doesn't get called when debugging in VS Code, see https://stackoverflow.com/questions/59765405/running-a-gdb-command-before-attaching-it-to-a-process-via-visual-studio-code
#if DEBUG
    printf("\nReceived signal ");
    switch (signo)
    {
    case SIGINT:
        printf("SIGINT");
        break;
    case SIGTERM:
        printf("SIGTERM");
        break;
    default:
        printf("[0x%x]", signo);
        break;
    }
    printf(".\n");
#else
    // See https://stackoverflow.com/questions/16891019/how-to-avoid-using-printf-in-a-signal-handler
    char *msg = "\nReceived signal.\n";
    write(fileno(stdout), msg, strlen(msg));
#endif
}

int shutdown()
{
    if (wd > 0)
    {
        printf("Closing inotify watch descriptor 0x%x.\n", wd);
        if (inotify_rm_watch(fd, wd) == -1)
        {
            char msg[256]; sprintf(&msg[0], "inotify_rm_watch 0x%x", wd);
            return eprintf(&msg[0], 0);
        }
    }

    if (fd > 0)
    {
        printf("Closing inotify file  descriptor 0x%x.\n", fd);
        if (close(fd) == -1)
        {
            char msg[256]; sprintf(&msg[0], "close 0x%x", fd);
            return eprintf(&msg[0], 0);
        }
    }

    return EXIT_SUCCESS;
}

int main(int argc, char *argv[])
{
    char* bin = "examplemount";
    if (argc > 0) {
        bin = argv[0];
    } else {
        printf("argc: %i\n", argc);
    }

    if (argc < 2) {
        printf("Usage: %s <folder>\n", bin);
        return 1;
    }

    fd = inotify_init();
    if (fd == -1)
    {
        return eprintf("inotify_init", 0);
    }

    char* path = argv[1];
    if (strcmp(path, ".") == 0) {
        path = getcwd(NULL, 0);
    }

    wd = inotify_add_watch(fd, path, IN_ALL_EVENTS);
    if (wd == -1)
    {
        char msg[256]; sprintf(&msg[0], "inotify_add_watch \"%s\"", path);

        int res = eprintf(&msg[0], 0);

        shutdown();

        return res;
    }

    // See https://man7.org/linux/man-pages/man7/signal.7.html and https://stackoverflow.com/questions/6249577/interrupting-blocked-read
    struct sigaction act = { 0 };
    //act.sa_flags = SA_SIGINFO | SA_UNSUPPORTED | SA_EXPOSE_TAGBITS;
    act.sa_sigaction = &sighandle;
    if (sigaction(SIGINT, &act, NULL) == -1) {
        int res = eprintf("sigaction SIGINT", 0);

        shutdown();

        return res;
    }

    if (sigaction(SIGTERM, &act, NULL) == -1) {
        int res = eprintf("sigaction SIGTERM", 0);

        shutdown();

        return res;
    }

    int pid = getpid();
    printf("PID %i monitoring \"%s\" with inotify file descriptor 0x%x and inotify watch descriptor 0x%x.\n", pid, path, fd, wd);

    char buf[BUF_LEN] __attribute__ ((aligned(8)));
    for (;;) {
        ssize_t numRead = read(fd, buf, BUF_LEN);

        if (numRead == -1)
        {
            int res = eprintf("read", 0);

            if (res == EINTR)
            {
                break;
            }

            shutdown();

            return res;
        }

        printf("Read %ld bytes from inotify file descriptor 0x%x.\n", (long) numRead, fd);

        if (numRead == 0)
        {
            continue;
        }

        /* Process all of the events in buffer returned by read() */

        char *p;
        struct inotify_event *event;
        for (p = buf; p < buf + numRead; ) {
            event = (struct inotify_event *) p;
            displayInotifyEvent(event);

            p += sizeof(struct inotify_event) + event->len;
        }
    }

    return shutdown();
}