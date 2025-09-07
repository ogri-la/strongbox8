#!/bin/bash
# build an AppImage of Strongbox
# run within the debian container. see Dockerfile and build-image.sh
set -eu

git config --global --add safe.directory /app/atk
git config --global --add safe.directory /app

echo
echo "--- building app ---"

# todo:
#rm -f resources/full-catalogue.json
#wget https://raw.githubusercontent.com/ogri-la/strongbox-catalogue/master/full-catalogue.json \
#    --quiet \
#    --directory-prefix resources
./manage.sh release
test -d "./release"

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

./linuxdeploy \
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
    appimagetool squashfs-root strongbox.AppImage.upx

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
