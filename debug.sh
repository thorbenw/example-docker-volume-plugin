#!/bin/sh

# Usage: debug.sh [name] [logfile]
#
# Creates the plugin,  opens a shell command prompt for testing, and finally
# cleans up the plugin, and threfore provides what later in this text is
# refrerred to as `debug sessions`.
#
#  name     The plugin name to use throughout testing.
#           Defaults to the current directory name.
#
#  logfile  The file to use as the source for messages from the plugin's STDOUT
#           stream. All lines containing the plugin id are sent to the console.
#           Default: /var/log/syslog
#
# All options are positional, so in order to use the default for an option, just
# provide an empty string ("") in it's position.
#
# If the plugin is already installed, it will be removed first, and then a fresh
# install of the pluigin will be `create`d.
# To stop the debug session, simply `exit` the prompt. If you want the plugin to
# remain and being retained after testing, leave the prompt with a non-zero 
# return code (e.g. type `exit 1`). The script will stop immediately afterwards.
#
# When using docker desktop, the context usually is 'default-linux', and the log
# file is '~/.docker/desktop/log/host/com.docker.backend.log'.
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
if [ -z "$docker_container_daemon_log" ]; then
    docker_container_daemon_log=/var/log/syslog
    case "$docker_context" in
        default)
            ;;
        desktop-linux)
            docker_container_daemon_log=~/.docker/desktop/log/host/com.docker.backend.log ;;
        *)
            echo "There is no specific default log file known for docker context [$docker_context]. Retaining generic default [$docker_container_daemon_log]."
    esac
fi
echo "Using log file [$docker_container_daemon_log]."

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
        exit 1
    fi
fi

docker plugin create "$plugin_name" . >/dev/null || exit $?
echo "Created  plugin [$plugin_name]."
id=$(docker plugin inspect --format '{{.Id}}' "$plugin_name")
if [ ! -z "$id" ]; then
    echo "Plugin [$plugin_name] has id [$id]."
else
    echo "Failed to get id of plugin [$plugin_name]."
    exit 2
fi

docker plugin enable --timeout 5 "$plugin_name" >/dev/null
res=$?
if [ $res -gt 0 ]; then
    cat "$docker_container_daemon_log" | grep $id
    echo "Failed to enable plugin [$plugin_name]."
    exit $res
fi
echo "Enabled  plugin [$plugin_name]."

cat "$docker_container_daemon_log" | grep $id
tail -f -n 0 "$docker_container_daemon_log" | grep $id &

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

echo "Temporarily exported environment variables available during debug session: $(printenv | grep ^plugin)"
echo "Starting shell [$prompt]. Type 'exit' to stop debugging. Use the \$plugin variable to refer to the plugin."
"$prompt"
res=$?
echo "Stopping shell [$prompt] to stop debugging."
cd "$lcd"
if [ $res -gt 0 ]; then
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
