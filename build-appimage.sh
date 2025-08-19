#!/bin/bash
# build an AppImage of Strongbox
# intended to run within a debian container. see Dockerfile and build-image.sh
set -eux


echo
echo "--- building app ---"

./manage.sh clean
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
    upx --best squashfs-root/usr/bin/linux-amd64
    appimagetool squashfs-root strongbox-compressed.AppImage
)

echo
echo "--- cleaning up ---"
rm -rf AppDir

echo
echo "done."
