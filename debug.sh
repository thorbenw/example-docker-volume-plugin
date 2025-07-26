#!/bin/sh

# Usage: debug.sh [name] [[+].|-|logfile]
#
# Creates the plugin,  opens a shell command prompt for testing, and finally
# cleans up the plugin, and threfore provides what later in this text is
# refrerred to as `debug sessions`.
#
#  name     The plugin name to use throughout testing.
#           Defaults to the current directory name.
#
#  logfile  The file to use as the source for messages from the plugin's STDOUT
#           stream.
#           If no file is specified, a list of default file names is probed and
#           the first file actually existing is used. If none of the probed
#           files could be found, the debugging session ist started without
#           logging information! Otherwise, i.e. if a file has been specified,
#           but doesn't exist, operation is aborted. If only a single dot (.) is
#           specified as the log file name, the defaults will be used, but if
#           none of the probed files could be found, operation will also be
#           aborted. If a dash (-) is specified, no  log file will be used at
#           all, but the debugging session will be started anyway.
#           By default, only lines containing the plugin id are sent to the
#           console. However, if any of the above specifications is prefixed
#           with a plus sign (+), all lines from the log file are sent there.
#           Defaults: /var/log/syslog and /var/log/messages
#               Depending on the currently selected docker context, additional
#               files are added to the defaults list:
#                   context 'default-linux':
#                       '~/.docker/desktop/log/host/com.docker.backend.log'
#
# All options are positional, so in order to use the default for an option, just
# provide an empty string ("") in it's position.
#
# If the plugin is already installed, it will be removed first, and then a fresh
# install of the pluigin will be `create`d.
# To stop the debug session, simply `exit` the prompt. If you want the plugin to
# remain and being retained after testing, leave the prompt with a non-zero 
# return code (e.g. type `exit 1`). The script will only perform basic cleanup
# steps and stop immediately afterwards.
#
# This script can be reused for developing other plugins due to the plugin name
# not being hard coded. This also allows for multiple tests at the same time by
# using different names for the respective debug sessions.

docker_context=$(docker context show)
if [ -z "$docker_context" ]; then
    echo "Failed to determine context."
    exit 1
fi

plugin_build=0
plugin_rootfs="./rootfs"
if [ ! -d "$plugin_rootfs" ]; then
    plugin_build=1
fi
if [ $plugin_build -gt 0 ]; then
    ./build.sh "$plugin_rootfs" || exit $?
else
    echo "Current docker context is [$docker_context]."
fi

plugin_name="$( basename $( cd "$( dirname "$0" )" && pwd ) )"
plugin_name=${1:-$plugin_name}
plugin_name=${plugin_name}:dev

docker_container_daemon_log="$2"
if [ "$(echo "$docker_container_daemon_log" | sed 's/^\(.\).*/\1/')" = "+" ]; then
    docker_container_daemon_log="$(echo "$docker_container_daemon_log" | sed -e 's#.*+\(\)#\1#')"
    tail_nogrep=true
fi
if [ "$docker_container_daemon_log" = "." ]; then
    docker_container_daemon_log=""
    docker_container_daemon_log_require=true
fi
if [ -z "$docker_container_daemon_log" ]; then
    docker_container_daemon_logs="/var/log/syslog /var/log/messages"
    case "$docker_context" in
        default)
            ;;
        desktop-linux)
            docker_container_daemon_logs="~/.docker/desktop/log/host/com.docker.backend.log $docker_container_daemon_logs" ;;
        *)
            echo "There are no specific default log files known for docker context [$docker_context]. Retaining generic defaults."
    esac
    for docker_container_daemon_log in $docker_container_daemon_logs; do
        echo -n "Probing log file [$docker_container_daemon_log] ..."
        if [ -f "$docker_container_daemon_log" ]; then
            echo " found."
            break
        else
            echo " not found."
            docker_container_daemon_log=""
        fi
    done
else
    if [ "$docker_container_daemon_log" = "-" ]; then
        docker_container_daemon_log=""
    else
        if [ ! -f "$docker_container_daemon_log" ]; then
            echo "Log file [$docker_container_daemon_log] has been explicitly specified but doesn't exist. Aborting."
            exit 2
        fi
    fi
fi
if [ -z "$docker_container_daemon_log" ]; then
    echo -n "There is no log file available for this debug session"
    if [ -z "$docker_container_daemon_log_require" ]; then
        echo " (continuing anyway)."
    else
        echo ". Aborting."; exit 2
    fi
else
    echo -n "Using log file [$docker_container_daemon_log]"
    if [ -z "$tail_nogrep" ]; then
        echo " (only printing plugin related lines)."
    else
        echo "."
    fi
fi

id=$(docker plugin inspect --format '{{.Id}}' "$plugin_name" 2>/dev/null)
if [ ! -z "$id" ]; then
    echo "Cleanup  plugin [$plugin_name]."
    id=$(docker plugin disable -f "$plugin_name" 2>/dev/null)
    if [ ! -z "$id" ]; then
        echo "Disabled plugin [$id]."
    fi
    id=$(docker plugin rm -f "$plugin_name" 2>/dev/null)
    if [ ! -z "$id" ]; then
        echo "Removed  plugin [$id]."
    fi
    id=$(docker plugin inspect --format '{{.Id}}' "$plugin_name" 2>/dev/null)
    if [ ! -z "$id" ]; then
        echo "Cleaning up plugin [$plugin_name] failed."
        exit 3
    fi
fi

docker plugin create "$plugin_name" . >/dev/null || exit $?
echo "Created  plugin [$plugin_name]."
id=$(docker plugin inspect --format '{{.Id}}' "$plugin_name")
if [ ! -z "$id" ]; then
    echo "Plugin [$plugin_name] has id [$id]."
else
    echo "Failed to get id of plugin [$plugin_name]."
    exit 4
fi

docker plugin enable --timeout 5 "$plugin_name" >/dev/null
res=$?
if [ $res -gt 0 ]; then
    if [ -f "$docker_container_daemon_log" ]; then
        cat "$docker_container_daemon_log" | grep $id
    fi
    echo "Failed to enable plugin [$plugin_name]."
    exit $res
fi
echo "Enabled  plugin [$plugin_name]."

if [ -f "$docker_container_daemon_log" ]; then
    tail_pid_file=$(mktemp)
    cat "$docker_container_daemon_log" | grep $id
    if [ -z "$tail_nogrep" ]; then
        ( tail -f -n 0 "$docker_container_daemon_log" & echo $! >"$tail_pid_file") | grep $id &
    else
        ( tail -f -n 0 "$docker_container_daemon_log" & echo $! >"$tail_pid_file")
    fi
fi

export plugin_runtime_dir="/var/lib/docker/plugins/$id"
export plugin_context=$docker_context
export plugin_log=$docker_container_daemon_log
export plugin_id=$id
export plugin=$plugin_name
prompt=${SHELL:-sh}
lcd=$(pwd)
if [ -d "$plugin_runtime_dir" ]; then
    export PATH=$PATH:$lcd
    cd "$plugin_runtime_dir"
else
    echo "Plugin runtime folder [$plugin_runtime_dir] is not accessible for the current user. Can neither change there nor alter PATH. Continuing anyway. "
fi

echo "\nTemporarily exported environment variables available during debug session:\n$(printenv | grep ^plugin)"
echo "\nStarting shell [$prompt]. Type 'exit' to stop debugging. Use the \$plugin variable to refer to the plugin."
"$prompt"
res=$?
echo "Stopping shell [$prompt] to stop debugging and cleaning up state and processes."
cd "$lcd"

if [ -f "$tail_pid_file" ]; then
    tail_pid=$(cat "$tail_pid_file") || true
    rm "$tail_pid_file" || true
    echo "Terminating the tail process used to forward log file [$docker_container_daemon_log] to this console (pid=[$tail_pid])."
    kill -15 $tail_pid || echo "Failed terminating the tail process. Continuing anyway!\n"
fi

if [ $res -gt 0 ]; then
    echo "Shell [$prompt] exited with code [$res]. Aborting debug session cleanup."
    exit $res
fi

docker plugin disable -f "$plugin_name" >/dev/null || exit $?
echo "Disabled plugin [$plugin_name]."
docker plugin rm "$plugin_name" >/dev/null || exit $?
echo "Removed  plugin [$plugin_name]."
if [ $plugin_build -gt 0 ]; then
    rm -rf "$plugin_rootfs" 2>/dev/null || exit $?
    echo "Cleaned up [$plugin_rootfs]."
fi
