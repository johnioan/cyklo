#!/bin/sh
# Builds the Cyklo .spk (run from the repository root). Version comes from packaging/INFO.
set -e
V=$(sed -n 's/^version="\([0-9.]*\)-.*/\1/p' packaging/INFO)
W=$(mktemp -d)
mkdir -p "$W/pkg/bin" "$W/spk"
(cd app && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "-s -w -X main.version=$V" -o "$W/pkg/bin/cyklo" .)
cp -r packaging/ui "$W/pkg/ui"
cp -r packaging/INFO packaging/scripts packaging/conf packaging/PACKAGE_ICON.PNG packaging/PACKAGE_ICON_256.PNG "$W/spk/"
(cd "$W/pkg" && tar -czf ../spk/package.tgz --owner=0 --group=0 bin ui)
(cd "$W/spk" && tar -cf "$OLDPWD/cyklosub-$V.spk" --owner=0 --group=0 INFO package.tgz scripts conf PACKAGE_ICON.PNG PACKAGE_ICON_256.PNG)
echo "Built cyklosub-$V.spk"
