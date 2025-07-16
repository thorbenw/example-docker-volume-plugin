#!/bin/sh

# Opens an interactive terminal in the container the plugin is running in. This
# lets developers e.g. inspect the fiel system 'inside' the plugin, especially
# when using propagated mounts in a plugin, this can be quite helpful.

id=${1:-$plugin_id}
if [ -z "$id" ]; then
    echo "Failed to retrieve the container id to use, and none has been specified."
fi

runc --root /run/docker/runtime-runc/plugins.moby exec -t "$id" sh
