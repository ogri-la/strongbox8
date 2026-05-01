#!/bin/bash
# generate binaries suitable for releasing to the public.
# this should be done within a controlled environment like a container (see ./build-image.sh)

set -euo pipefail

if [ -z ${IS_DOCKER+x} ]; then
    echo "run this inside a container with ./manage.sh release"
    exit 1
fi

required_tools=(
    "go"
    "linuxdeploy"
    "upx"
    "appimagetool"
    "aarch64-linux-gnu-gcc"
    "x86_64-w64-mingw32-gcc"
    #"aarch64-w64-mingw32-gcc"
)
for tool in "${required_tools[@]}"; do
    if ! command -v "$tool" &> /dev/null; then
        echo "ERROR: '$tool' not found, cannot continue"
        exit 1
    fi
done

./manage.sh clean

git config --global --add safe.directory /app/atk
git config --global --add safe.directory /app

(
    cd ./atk
    current_branch=$(git rev-parse --abbrev-ref HEAD)
    if [[ "$current_branch" != "master" ]]; then
        echo "ERROR: ./atk is not on the 'master' branch, refusing to release: $current_branch"
        exit 1
    fi

    if [[ -n $(git ls-files --others --exclude-standard) ]]; then
        echo "ERROR: ./atk has untracked files, refusing to release"
        exit 1
    fi

    if [[ -n $(git status --porcelain) ]]; then
        echo "ERROR: ./atk has uncommitted changes, refusing to release"
        exit 1
    fi

    if [[ -n $(git cherry -v) ]]; then
        echo "ERROR: ./atk has unpushed commits, refusing to release"
        exit 1
    fi
)

# warn when the releases generated may not be reproducible.
current_branch=$(git rev-parse --abbrev-ref HEAD)
if [[ "$current_branch" != "master" ]]; then
    echo "WARNING: ./ is not on the 'master' branch: $current_branch"
fi

if [[ -n $(git ls-files --others --exclude-standard) ]]; then
    echo "WARNING: ./ has untracked files"
fi

if [[ -n $(git status --porcelain) ]]; then
    echo "WARNING: ./ has uncommitted changes"
fi

if [[ -n $(git cherry -v) ]]; then
    echo "WARNING: ./ has unpushed commits"
fi

# GOOS is 'Go OS' and is being explicit in which OS to build for.
# ld -s is 'disable symbol table'
# ld -w is 'disable DWARF generation'
# -trimpath removes leading paths to source files
# -v 'verbose'
# -o 'output'

platforms=("linux" "windows") # cannot target darwin without incorporating Xcode, which would violate GPL
architectures=("arm64" "amd64")

output_dir="release"
mkdir -p "$output_dir"

ldflags="-s -w"
trimpath="-trimpath"
cgo_enabled=1

pids=()

for GOOS in "${platforms[@]}"; do
    for GOARCH in "${architectures[@]}"; do
        (
            binary_name="${GOOS}-${GOARCH}" # "linux-amd64", "windows-arm64"
            output_path="${output_dir}/${binary_name}" # "./release/linux-amd64"

            unset CC
            if [[ "$GOOS" == "linux" && "$GOARCH" == "arm64" ]]; then
		export CC=aarch64-linux-gnu-gcc

            elif [[ "$GOOS" == "windows" && "$GOARCH" == "amd64" ]]; then
		export CC=x86_64-w64-mingw32-gcc
		output_path+=".exe"

            elif [[ "$GOOS" == "windows" && "$GOARCH" == "arm64" ]]; then
		export CC=aarch64-w64-mingw32-gcc
		output_path+=".exe"

            fi

            export CGO_ENABLED=$cgo_enabled
            export GOOS=$GOOS
            export GOARCH=$GOARCH

            echo "Building for $GOOS/$GOARCH..."

            go build \
               -C strongbox \
               -ldflags="$ldflags" \
               $trimpath \
               -o "$output_path"
            sha256sum "strongbox/$output_path" > "strongbox/${output_path}.sha256"

            if [[ "$GOARCH" == "amd64" ]]; then
                (
		    cd ./strongbox
		    upx --best "$output_path" -o "${output_path}.upx"
		    sha256sum "${output_path}.upx" > "${output_path}.upx.sha256"
                )
            fi
        ) &
        pids+=($!)
    done
done

failed=0
for pid in "${pids[@]}"; do
    if ! wait "$pid"; then
        failed=1
    fi
done

if [[ $failed -ne 0 ]]; then
    echo "ERROR: one or more builds failed, cannot continue to AppImage"
    exit 1
fi

rm -rf ./release
mv ./strongbox/release ./
ls -la ./release

# AppImage -----------------------------------------------------------------

echo
echo "--- building AppImage ---"

# todo:
#rm -f resources/full-catalogue.json
#wget https://raw.githubusercontent.com/ogri-la/strongbox-catalogue/master/full-catalogue.json \
    #    --quiet \
    #    --directory-prefix resources

rm -rf ./AppDir
mkdir -p AppDir
#mv "$custom_jre_dir" AppDir/usr
#mv ./release/linux-amd64 AppDir/
#cp ./AppImage/strongbox.desktop ./AppImage/strongbox.svg ./AppImage/strongbox.png ./AppImage/AppRun AppDir/
du -sh AppDir/
rm -f strongbox.appimage # safer than 'rm -f strongbox'

#ARCH=x86_64 ./appimagetool AppDir/ strongbox.appimage
#export NO_STRIP=1

mkdir -p AppDir/usr/share/
cp -R /usr/share/tcltk/ AppDir/usr/share/

linuxdeploy \
    --appdir AppDir \
    --custom-apprun AppImage/AppRun \
    --executable release/linux-amd64 \
    --desktop-file AppImage/strongbox.desktop \
    --icon-file AppImage/strongbox.svg \
    --icon-file AppImage/strongbox.png \
    --output appimage

mv Strongbox-x86_64.AppImage release/strongbox.AppImage
du -sh release/strongbox.AppImage

echo "--- upx"
(
    cd release
    ./strongbox.AppImage --appimage-extract
    cp ./linux-amd64.upx ./squashfs-root/usr/bin/linux-amd64
    ARCH=x86_64 appimagetool squashfs-root strongbox.AppImage.upx

    sha256sum "strongbox.AppImage" > "strongbox.AppImage.sha256"
    sha256sum "strongbox.AppImage.upx" > "strongbox.AppImage.upx.sha256"
    rm -rf ./squashfs-root
)

echo "--- uploadables"
(
    cd release
    rm -rf dist
    mkdir dist
    (
        cd dist
        ln -s ../linux-amd64.upx linux-amd64
        ln -s ../linux-amd64.upx.sha256 linux-amd64.sha256
        ln -s ../linux-arm64 linux-arm64
        ln -s ../linux-arm64.sha256 linux-arm64.sha256
        ln -s ../strongbox.AppImage.upx strongbox.AppImage
        ln -s ../strongbox.AppImage.upx.sha256 strongbox.AppImage.sha256
        ln -s ../windows-amd64.exe.upx windows-amd64.exe
        ln -s ../windows-amd64.exe.upx.sha256 windows-amd64.exe.sha256
        ln -s ../windows-arm64.exe windows-arm64.exe
        ln -s ../windows-arm64.exe.sha256 windows-arm64.exe.sha256
    )
)

echo
echo "--- cleaning up ---"
rm -rf AppDir
    
echo
echo "done."
