#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
#include <strings.h>
#include <signal.h>
#include <bits/types/sigset_t.h>

int signame(char* name)
{
    if (NULL == name)
    {
        return 0;
    }

    if (strcasecmp(name, "SIGHUP") == 0)
    {
        return SIGHUP;
    }

    if (strcasecmp(name, "SIGINT") == 0)
    {
        return SIGINT;
    }

    if (strcasecmp(name, "SIGQUIT") == 0)
    {
        return SIGQUIT;
    }

    if (strcasecmp(name, "SIGILL") == 0)
    {
        return SIGILL;
    }

    if (strcasecmp(name, "SIGTRAP") == 0)
    {
        return SIGTRAP;
    }

    if (strcasecmp(name, "SIGABRT") == 0)
    {
        return SIGABRT;
    }

    if (strcasecmp(name, "SIGFPE") == 0)
    {
        return SIGFPE;
    }

    if (strcasecmp(name, "SIGKILL") == 0)
    {
        printf("SIGKILL is unblockable!\n");
        return 0; // SIGKILL = 9
    }

    if (strcasecmp(name, "SIGSTOP") == 0)
    {
        printf("SIGSTOP is unblockable!\n");
        return 0; // SIGSTOP = 19
    }

    return 0;
}

int main(int argc, char *argv[])
{
    sigset_t sigset;
    (void) sigemptyset(&sigset);

    for (int i = 1; i < argc; i++) {
        int addset = signame(argv[i]);
        if (addset > 0) 
        {
            printf("Adding   set argv[%i] = %s.\n", i, argv[i]);
        }
        else
        {
            addset = atoi(argv[i]);
            if (addset > 0) 
            {
                printf("Adding   set argv[%i] = %i.\n", i, addset);
            }
        }

        if (addset < 1) 
        {
            printf("Skipping set argv[%i] = [%s].\n", i, argv[i]);
            continue;
        }

        (void) sigaddset (&sigset, addset);
    }

    int pid = getpid();
    while (1) {
        (void) printf("PID %i running, waiting for signals ...\n", pid);
        (void) sigsuspend(&sigset);
    }
}
