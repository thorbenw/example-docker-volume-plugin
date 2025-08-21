#/bin/sh

# Usage: build.sh [path] [version]
#
# Creates either a `rootfs` folder in the current folder (which is meant to be
# the project folder of the plugin project to build!), or a tarball for
# deployment.
#
#  path     The path to build to.
#
#           If it ends with `.tgz`, a (compressed) tarball is created.
#
#           Default: ./rootfs
#
#  version  The version information to embed by passing it to the MODULE_VERSION
#           ARGument in the docker file.
#
#           If a file with a name equaling this expression exists, it's contents
#           will be loaded and used as the version information.
#
#           Default: (build) - but if a file `config.ver` exists, it's contents
#                              are loaded as the default vlaue.
#
# When built to a `rootfs` folder, or after having extracted the tarball to a
# `rootfs` folder, as soon as a valid `config.json` file also exists in the same
# directory the `rootfs` folder resides in, the plugin can be `create`d directly
# from that folder.
#
# Remarks
#  - Other files present in the same folder as the `rootfs` folder and the
#    `config.json` file do NOT interfere or otherwise disturb the plugin
#    creation process.
#  - The `./src` folder is the build context root folder.
#  - A valid `Dockerfile` must exist in the build context in order to build a
#    temporary image and container to generate the $path folder or tarball from.

build_path=${1:-"./rootfs"}
build_target="rootfs"
image_name="rootfsimage"
image_tag="dev"

if (echo "$build_path" | grep -qi '.tgz$'); then
    build_target="tgz"
fi

case $build_target in
    tgz)
        echo "Building to tarball [$build_path]."
        if [ -f "$build_path" ]; then
            echo "Erasing existing tarball [$build_path]."
            rm -rf "$build_path" 2>/dev/null
            res=$?
            if [ $res -gt 0 ]; then
                echo "Failed to remove tarball. Aborting."
                exit $res
            fi
        fi
    ;;
    rootfs)
        echo "Building to folder [$build_path]."
        if [ -d "$build_path" ]; then
            echo "Erasing existing root file system [$build_path]."
            rm -rf "$build_path" 2>/dev/null
            res=$?
            if [ $res -gt 0 ]; then
                echo "Failed to remove root file system. Aborting."
                exit $res
            fi
        fi
    ;;
    *)
        >&2 echo "Build target [$build_target] is not (yet?) supported."
        exit 127
    ;;
esac

version_string=${2:-"(build)"}
version_file=${2:-"./config.ver"}
if [ -f "$version_file" ]; then
    echo "Loading version information from file [$version_file]."
    version_string=$(cat "$version_file")
fi
echo "Build version is [$version_string]."

if command -v go >/dev/null; then
    echo "Checking [go.sum]."
    cur_go_sum="./src/go.sum"
    tmp_go_sum="$(mktemp)"

    cp -p "$cur_go_sum" "$tmp_go_sum"
    go_sum_hash_old="$(sha256sum -b ./src/go.sum)"
    (cd ./src; go mod tidy)
    go_sum_hash_new="$(sha256sum -b ./src/go.sum)"
    mv "$tmp_go_sum" "$cur_go_sum"

    if [ "$go_sum_hash_old" != "$go_sum_hash_new" ]; then
        echo "Checked  [go.sum] -> File is outdated, please update."
        exit 1
    fi
    echo "Checked  [go.sum] -> Ok."
else
    echo "Cannot check go.sum due to go being absent. Errors may occur during build!"
fi

image_name="$image_name:$image_tag"
unset image_tag

echo "Starting build to image [$image_name]."
docker build -t "$image_name" --build-arg MODULE_VERSION="$version_string" --progress plain ./src || exit $?
id=$(docker create "$image_name" true)
if [ -z "$id" ]; then
    echo "Failed to create container or to get it's id."
    exit 127
fi

case $build_target in
    tgz)
        echo "Exporting to tarball [$build_path]."
        docker export "$id" | gzip -c9 > "$build_path" || exit $?
    ;;
    rootfs)
        echo "Exporting to root file system [$build_path]."
        mkdir -p "$build_path"
        docker export "$id" | tar -x -C "$build_path" || exit $?
    ;;
    *)
        >&2 echo "Build target [$build_target] is not supported."
    ;;
esac

docker rm -vf "$id" || exit $?
docker rmi "$image_name" || exit $?
