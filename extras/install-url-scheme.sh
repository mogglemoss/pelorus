#!/bin/bash
# Install: bash extras/install-url-scheme.sh
set -e

APP_DIR="$HOME/Applications/Pelorus URL Handler.app"
PELORUS_BIN=$(command -v pelorus 2>/dev/null || echo "$HOME/go/bin/pelorus")

echo "Installing Pelorus URL scheme handler..."
echo "  pelorus binary: $PELORUS_BIN"
echo "  app location:   $APP_DIR"
echo ""

# Write the AppleScript source.
SCRIPT=$(mktemp /tmp/pelorus-handler-XXXXXX.applescript)
cat > "$SCRIPT" << 'APPLESCRIPT'
on open location this_URL
    -- Strip "pelorus://" prefix (10 chars).
    set thePath to text 11 thru -1 of this_URL
    -- Basic URL decode: replace %20 with space (covers common cases).
    set thePath to do shell script "python3 -c \"import urllib.parse, sys; print(urllib.parse.unquote(sys.argv[1]))\" " & quoted form of thePath
    set theCmd to "pelorus " & quoted form of thePath
    tell application "Terminal"
        activate
        if (count windows) = 0 then
            do script theCmd
        else
            do script theCmd in front window
        end if
    end tell
end open location

on run
    tell application "Terminal"
        activate
        do script "pelorus"
    end tell
end run
APPLESCRIPT

# Compile to .app bundle (osacompile creates the full bundle structure).
osacompile -o "$APP_DIR" "$SCRIPT"
rm -f "$SCRIPT"

# Add CFBundleURLTypes to the compiled app's Info.plist.
PLIST="$APP_DIR/Contents/Info.plist"
/usr/libexec/PlistBuddy -c "Delete :CFBundleURLTypes" "$PLIST" 2>/dev/null || true
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes array" "$PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0 dict" "$PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLName string 'Pelorus File Manager'" "$PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLSchemes array" "$PLIST"
/usr/libexec/PlistBuddy -c "Add :CFBundleURLTypes:0:CFBundleURLSchemes:0 string 'pelorus'" "$PLIST"

# Register with Launch Services so macOS knows about the scheme.
LSREGISTER=/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister
"$LSREGISTER" -f "$APP_DIR"

echo "Done. Test with:"
echo "  open 'pelorus:///Users/$USER/Documents'"
echo ""
echo "If the handler does not open, log out and back in, or run:"
echo "  $LSREGISTER -kill -r -domain local -domain system -domain user && $LSREGISTER -f \"$APP_DIR\""
