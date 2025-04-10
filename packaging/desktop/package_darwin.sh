#!/usr/bin/env bash

set -e -u -o pipefail # Fail on error

dir=$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )
cd $dir

client_dir="$dir/../.."
build_dir=${BUILD_DIR:-"/tmp/package_darwin/build"}
save_dir=${SAVE_DIR:-}
tmp_dir="/tmp/package_darwin/tmp"
bucket_name=${BUCKET_NAME:-}
run_mode="prod"
platform="darwin"
nosign=${NOSIGN:-}

s3host=""
if [ ! "$bucket_name" = "" ]; then
  # Use this syntax since bucket_name might have dots (.)
  s3host="https://s3.amazonaws.com/$bucket_name"
fi

# Ensure we have packaging tools
npm install
node_bin="$dir/node_modules/.bin"

app_name=Keybase
keybase_version=${KEYBASE_VERSION:-}
kbfs_version=${KBFS_VERSION:-}
comment=""

keybase_binpath=${KEYBASE_BINPATH:-}
kbfs_binpath=${KBFS_BINPATH:-}

icon_path="$client_dir/media/icons/Keybase.icns"

echo "Loading release tool"
"$client_dir/packaging/goinstall.sh" "github.com/keybase/release"
release_bin="$GOPATH/bin/release"

if [ "$keybase_version" = "" ]; then
  if [ ! "$keybase_binpath" = "" ]; then
    keybase_version=`$keybase_binpath version -S`
    echo "Using keybase (bin) version: $keybase_version"
  fi
fi

if [ "$kbfs_version" = "" ]; then
  if [ ! "$kbfs_binpath" = "" ]; then
    kbfs_version=`$kbfs_binpath -version`
    echo "Using kbfs (bin) version: $kbfs_version"
  fi
fi

if [ "$keybase_version" = "" ]; then
  echo "Specify KEYBASE_VERSION to use (Github release/tag)"
  exit 1
fi

if [ "$kbfs_version" = "" ]; then
  echo "Specify KBFS_VERSION for use (Github release/tag)"
  exit 1
fi

# if [ "$comment" = "" ]; then
#   comment=`git rev-parse --short HEAD`
#   echo "Using comment: $comment"
# fi
# comment="+$comment"

out_dir="$build_dir/Keybase-darwin-x64"
shared_support_dir="$out_dir/Keybase.app/Contents/SharedSupport"
resources_dir="$out_dir/Keybase.app/Contents/Resources/"

installer_url="https://github.com/keybase/client/releases/download/v1.0.14-0/KeybaseInstaller-1.1.27-darwin.tgz"

keybase_bin="$tmp_dir/keybase"
kbfs_bin="$tmp_dir/kbfs"
installer_app="$tmp_dir/KeybaseInstaller.app"

app_version="$keybase_version"
dmg_name="${app_name}-${app_version}${comment}.dmg"
zip_name="${app_name}-${app_version}${comment}.zip"
sourcemap_name="${app_name}-${app_version}${comment}.map.zip"
sig_name="${app_name}-${app_version}${comment}.sig"
update_json_name="update-${platform}-${run_mode}-${app_version}.json"

clean() {
  echo "Cleaning"
  rm -rf "$build_dir"
  rm -rf "$tmp_dir"
  mkdir -p "$build_dir"
  mkdir -p "$tmp_dir"
}

ensure_url() {
  url="$1"
  msg="$2"
  if ! curl --output /dev/null --silent --head --fail "$url"; then
    echo "URL doesn't exist: $url"
    echo "$msg"
    exit 1
  fi
}

get_deps() {(
  cd "$tmp_dir"
  echo "Downloading dependencies"

  if [ ! "$keybase_binpath" = "" ]; then
    echo "Using local keybase binpath: $keybase_binpath"
    cp "$keybase_binpath" .
  else
    keybase_url="https://github.com/keybase/client/releases/download/v$keybase_version/keybase-$keybase_version-darwin.tgz"
    echo "Getting $keybase_url"
    ensure_url "$keybase_url" "You need to build the binary for this Github release/version. See packaging/github to create/build a release."
    curl -J -L -Ss "$keybase_url" | tar zx
  fi

  if [ ! "$kbfs_binpath" = "" ]; then
    echo "Using local kbfs binpath: $kbfs_binpath"
    cp "$kbfs_binpath" .
  else
    kbfs_url="https://github.com/keybase/kbfs-beta/releases/download/v$kbfs_version/kbfs-$kbfs_version-darwin.tgz"
    echo "Getting $kbfs_url"
    ensure_url $kbfs_url "You need to build the binary for this Github release/version. See packaging/github to create/build a release."
    curl -J -L -Ss "$kbfs_url" | tar zx
  fi
  echo "Using installer from $installer_url"
  curl -J -L -Ss "$installer_url" | tar zx
)}

# Build Keybase.app
package_electron() {(
  cd "$client_dir/desktop"

  rm -rf node_modules
  npm install
  npm run package -- --appVersion="$app_version" --comment="$comment" --icon="$icon_path"
  rsync -av release/darwin-x64/Keybase-darwin-x64 "$build_dir"
)}

# Adds the keybase binaries and Installer.app bundle to Keybase.app
package_app() {(
  cd "$build_dir"
  echo "Copying keybase binaries"
  mkdir -p "$shared_support_dir/bin"
  cp "$keybase_bin" "$shared_support_dir/bin"
  cp "$kbfs_bin" "$shared_support_dir/bin"
  echo "Copying installer"
  mkdir -p "$resources_dir"
  cp -R "$installer_app" "$resources_dir/KeybaseInstaller.app"
)}

update_plist() {(
  cd "$out_dir"
  # App shouldn't display dock icon on startup
  /usr/libexec/plistBuddy -c "Add :LSUIElement bool true" "$app_name.app/Contents/Info.plist"
  /usr/libexec/plistBuddy -c "Add :NSSupportsSuddenTermination bool true" "$app_name.app/Contents/Info.plist"
)}

sign() {(
  cd "$out_dir"
  if [ "$nosign" = "1" ]; then
    echo "Skipping signing (OS X)"
    return
  fi
  code_sign_identity="Developer ID Application: Keybase, Inc. (99229SGT5K)"
  codesign --verbose --force --deep --timestamp=none --sign "$code_sign_identity" "$app_name.app"
)}

# Create dmg from Keybase.app
package_dmg() {(
  cd "$out_dir"
  appdmg="appdmg.json"

  osx_scripts="$client_dir/osx/Scripts"
  cp "$osx_scripts/appdmg/$appdmg" .
  cp "$osx_scripts/appdmg/Background.png" .
  cp "$icon_path" .

  rm -rf "$dmg_name"
  "$node_bin/appdmg" "$appdmg" "$dmg_name"
)}

create_sourcemap_zip() {(
  cd "$out_dir"
  echo "Creating $sourcemap_name from $client_dir/desktop/dist"
  zip -j "$sourcemap_name" "$client_dir/desktop/dist"/*.map
)}

create_zip() {(
  cd "$out_dir"
  echo "Creating $zip_name"
  #zip -r $zip_name $app_name.app
  ditto -c -k --sequesterRsrc --keepParent "$app_name.app" "$zip_name"
)}

kbsign() {(
  cd "$out_dir"
  keybase sign -d -i "$zip_name" -o "$sig_name"
)}

update_json() {(
  cd "$out_dir"
  if [ -n "$s3host" ]; then
    "$release_bin" update-json --version="$app_version" --src="$zip_name" \
      --uri="$s3host/$platform-updates" --signature="$out_dir/$sig_name" > "$update_json_name"
  fi
)}

save() {(
  cd "$out_dir"
  if [ "$save_dir" = "" ]; then
    echo "Saved files to $out_dir"
    return
  fi
  mkdir -p $save_dir
  cd "$save_dir"
  platform_dir="$save_dir/$platform"
  echo "Saving files to $platform_dir"
  # DMG
  mkdir -p "$platform_dir"
  mv "$out_dir/$dmg_name" "$platform_dir"
  # Zip
  mkdir -p "$platform_dir-updates"
  mv "$out_dir/$zip_name" "$platform_dir-updates"
  # Sourcemap
  mkdir -p "$save_dir/electron-sourcemaps"
  mv "$out_dir/$sourcemap_name" "$save_dir/electron-sourcemaps"
  # Support files
  mkdir -p "$platform_dir-support"
  mv "$out_dir/$update_json_name" "$platform_dir-support"
)}

s3sync() {
  if [ ! "$bucket_name" = "" ] && [ ! "$save_dir" = "" ]; then
    s3cmd sync --acl-public --disable-multipart $save_dir/* s3://$bucket_name/
  else
    echo "S3 sync disabled"
  fi
}

clean
get_deps
package_electron
package_app
update_plist
sign
package_dmg
create_sourcemap_zip
create_zip
kbsign
update_json
save
s3sync
