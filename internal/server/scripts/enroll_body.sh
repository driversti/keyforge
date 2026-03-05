# Generate SSH key if needed
if [ ! -f "$KEY_PATH" ]; then
    echo "Generating SSH key pair..."
    mkdir -p "$(dirname "$KEY_PATH")"
    ssh-keygen -t ed25519 -f "$KEY_PATH" -N "" -q
    echo "Key pair generated at $KEY_PATH"
else
    echo "Using existing SSH key at $KEY_PATH"
fi

# Read public key
PUB_KEY=$(cat "${KEY_PATH}.pub")

# Register with server
echo "Registering device '$NAME' with KeyForge..."
RESPONSE=$(curl -sS -w "\n%{http_code}" -X POST "$SERVER_URL/api/v1/devices" \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"$NAME\",\"public_key\":\"$PUB_KEY\",\"accepts_ssh\":${ACCEPT_SSH:-false},\"enrollment_token\":\"$TOKEN\"}")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | head -n -1)

if [ "$HTTP_CODE" != "201" ]; then
    echo "Registration failed ($HTTP_CODE): $BODY"
    exit 1
fi

echo "Device '$NAME' enrolled successfully!"

# If accept-ssh, install authorized keys
if [ "$ACCEPT_SSH" = "true" ]; then
    echo "Fetching authorized keys..."
    KEYS=$(curl -sS "$SERVER_URL/api/v1/authorized_keys")

    HEADER="# --- KeyForge Managed Keys (DO NOT EDIT) ---"
    FOOTER="# --- End KeyForge Managed Keys ---"
    AUTH_FILE="$HOME/.ssh/authorized_keys"

    # Remove existing managed section if present
    if [ -f "$AUTH_FILE" ] && grep -q "$HEADER" "$AUTH_FILE"; then
        sed -i.bak "/$HEADER/,/$FOOTER/d" "$AUTH_FILE"
        rm -f "$AUTH_FILE.bak"
    fi

    # Append managed section
    printf "\n%s\n%s\n%s\n" "$HEADER" "$KEYS" "$FOOTER" >> "$AUTH_FILE"
    chmod 600 "$AUTH_FILE"
    echo "Authorized keys installed to $AUTH_FILE"

    # Set up cron if requested
    if [ -n "$SYNC_INTERVAL" ]; then
        # Convert interval to cron expression (POSIX-compatible)
        UNIT=$(echo "$SYNC_INTERVAL" | sed 's/.*\(.\)$/\1/')
        VALUE=$(echo "$SYNC_INTERVAL" | sed 's/.$//')

        case "$UNIT" in
            m)
                CRON_SCHEDULE="*/$VALUE * * * *"
                ;;
            h)
                CRON_SCHEDULE="0 */$VALUE * * *"
                ;;
            *)
                echo "Error: sync interval must end with 'm' (minutes) or 'h' (hours)"
                exit 1
                ;;
        esac
        CRON_EXPR="$CRON_SCHEDULE"
        CRON_LINE="$CRON_EXPR curl -sS $SERVER_URL/api/v1/authorized_keys > /tmp/kf_keys && (sed -i.bak '/$HEADER/,/$FOOTER/d' $AUTH_FILE 2>/dev/null; rm -f ${AUTH_FILE}.bak; printf '\\n$HEADER\\n' >> $AUTH_FILE; cat /tmp/kf_keys >> $AUTH_FILE; printf '$FOOTER\\n' >> $AUTH_FILE; rm /tmp/kf_keys)"

        (crontab -l 2>/dev/null | grep -v "KeyForge"; echo "$CRON_LINE") | crontab -
        echo "Cron job installed: sync every $SYNC_INTERVAL"
    fi
fi

echo "Done!"
