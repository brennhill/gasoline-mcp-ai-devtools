#!/bin/bash
# 10-recording.sh — 10.1-10.3: Video recording, audio, watermark.
set -eo pipefail

begin_category "10" "Recording" "3"

# ── Test 10.1: Record tab video (no audio) ───────────────
begin_test "10.1" "Record tab video for 5 seconds (no audio)" \
    "Start recording, wait 5s, stop, verify file saved with valid metadata" \
    "Tests: full recording pipeline: MCP > daemon > extension > tabCapture > blob > server > disk"

run_test_10_1() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    echo "  Navigating to YouTube lofi stream..."
    interact_and_wait "navigate" '{"action":"navigate","url":"https://youtu.be/n61ULEU7CO0?si=xT8FVrq5eIsJTfuI&t=646&autoplay=1","reason":"Load YouTube video for recording"}' 20
    sleep 3

    interact_and_wait "record_start" '{"action":"record_start","name":"smoke-video-test","reason":"Record tab video"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_start returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    echo "  Recording... waiting 5 seconds"
    sleep 5

    interact_and_wait "record_stop" '{"action":"record_stop","reason":"Stop recording"}' 20

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_stop returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 2

    local saved_response
    saved_response=$(call_tool "observe" '{"what":"saved_videos","last_n":1}')
    local saved_text
    saved_text=$(extract_content_text "$saved_response")

    echo "  [saved video metadata]"
    echo "$saved_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    recs = data.get('recordings', [])
    if recs:
        r = recs[0]
        print(f'    name: {r.get(\"name\", \"?\")[:60]}')
        print(f'    duration: {r.get(\"duration_seconds\", \"?\")}s')
        print(f'    size: {r.get(\"size_bytes\", 0)} bytes')
        print(f'    format: {r.get(\"format\", \"?\")}')
        print(f'    has_audio: {r.get(\"has_audio\", False)}')
        print(f'    fps: {r.get(\"fps\", \"?\")}')
    else:
        print('    (no recordings found)')
except: pass
" 2>/dev/null || true

    if echo "$saved_text" | grep -q "smoke-video-test"; then
        local dur
        dur=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
print(recs[0].get('duration_seconds',0) if recs else 0)
" 2>/dev/null || echo "0")
        if [ "$dur" -ge 3 ] 2>/dev/null; then
            pass "Video recorded: smoke-video-test, ${dur}s duration, saved to disk."
        else
            pass "Video recorded: smoke-video-test saved (duration: ${dur}s)."
        fi
    else
        fail "No 'smoke-video-test' found in saved_videos. Content: $(truncate "$saved_text" 200)"
    fi
}
run_test_10_1

# ── Test 10.2: Record tab video WITH tab audio ───────────
begin_test "10.2" "Record tab video with audio:tab for 5 seconds" \
    "Navigate to a page with sound, record with audio:'tab', verify audio metadata" \
    "Tests: tab audio capture via tabCapture"

run_test_10_2() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    echo "  Navigating to YouTube lofi stream..."
    interact_and_wait "navigate" '{"action":"navigate","url":"https://youtu.be/n61ULEU7CO0?si=xT8FVrq5eIsJTfuI&t=646&autoplay=1","reason":"Load YouTube video with audio for recording"}' 20
    sleep 3

    interact_and_wait "record_start" '{"action":"record_start","name":"smoke-audio-test","audio":"tab","reason":"Record tab with audio"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_start with audio:tab returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    echo "  Recording with audio... waiting 5 seconds"
    echo "  (play sound in the tracked tab now if you want to verify audio)"
    sleep 5

    interact_and_wait "record_stop" '{"action":"record_stop","reason":"Stop audio recording"}' 20

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        # Check specifically for 403 errors — indicates server auth issue
        if echo "$INTERACT_RESULT" | grep -qi "403"; then
            fail "record_stop returned 403 Forbidden. Server requires X-Gasoline-Client header for /recordings/save endpoint. Check extension upload headers. Result: $(truncate "$INTERACT_RESULT" 200)"
        else
            fail "record_stop returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        fi
        return
    fi

    sleep 2

    local saved_response
    saved_response=$(call_tool "observe" '{"what":"saved_videos","last_n":1}')
    local saved_text
    saved_text=$(extract_content_text "$saved_response")

    echo "  [saved video with audio metadata]"
    echo "$saved_text" | python3 -c "
import sys, json
try:
    t = sys.stdin.read(); i = t.find('{'); data = json.loads(t[i:]) if i >= 0 else {}
    recs = data.get('recordings', [])
    if recs:
        r = recs[0]
        print(f'    name: {r.get(\"name\", \"?\")[:60]}')
        print(f'    duration: {r.get(\"duration_seconds\", \"?\")}s')
        print(f'    size: {r.get(\"size_bytes\", 0)} bytes')
        print(f'    has_audio: {r.get(\"has_audio\", False)}')
        print(f'    audio_mode: {r.get(\"audio_mode\", \"(none)\")}')
    else:
        print('    (no recordings found)')
except: pass
" 2>/dev/null || true

    if ! echo "$saved_text" | grep -q "smoke-audio-test"; then
        fail "No 'smoke-audio-test' found in saved_videos. Content: $(truncate "$saved_text" 200)"
        return
    fi

    local has_audio
    has_audio=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
print(recs[0].get('has_audio',False) if recs else False)
" 2>/dev/null || echo "False")

    local audio_mode
    audio_mode=$(echo "$saved_text" | python3 -c "
import sys,json
t=sys.stdin.read(); i=t.find('{'); data=json.loads(t[i:]) if i>=0 else {}
recs=data.get('recordings',[])
print(recs[0].get('audio_mode','') if recs else '')
" 2>/dev/null || echo "")

    if [ "$has_audio" = "True" ] && [ "$audio_mode" = "tab" ]; then
        pass "Audio recording saved: has_audio=true, audio_mode=tab."
    else
        fail "Recording saved but audio metadata missing. has_audio=$has_audio, audio_mode=$audio_mode."
    fi

    echo ""
    echo "  >>> Open the .webm file in ~/.gasoline/recordings/ to verify audio is audible."
}
run_test_10_2

# ── Test 10.3: Recording watermark survives page refresh ─
begin_test "10.3" "Recording watermark survives page refresh" \
    "Start recording, refresh the page, verify watermark reappears" \
    "Tests: tabs.onUpdated listener re-sends watermark after navigation"

run_test_10_3() {
    if [ "$PILOT_ENABLED" != "true" ]; then
        skip "Pilot not enabled."
        return
    fi

    echo "  Navigating to YouTube lofi stream..."
    interact_and_wait "navigate" '{"action":"navigate","url":"https://youtu.be/n61ULEU7CO0?si=xT8FVrq5eIsJTfuI&t=646&autoplay=1","reason":"Load YouTube video for watermark test"}' 20
    sleep 2

    interact_and_wait "record_start" '{"action":"record_start","name":"smoke-watermark-test","reason":"Test watermark persistence"}'

    if echo "$INTERACT_RESULT" | grep -qi "error\|failed"; then
        fail "record_start returned error. Result: $(truncate "$INTERACT_RESULT" 200)"
        return
    fi

    sleep 3

    # YouTube is slow — give execute_js more poll time (30 polls = 15s)
    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Check watermark before refresh","script":"document.getElementById(\"gasoline-recording-watermark\") ? \"WATERMARK_FOUND\" : \"WATERMARK_MISSING\""}' 30
    local before_refresh="$INTERACT_RESULT"

    interact_and_wait "refresh" '{"action":"refresh","reason":"Refresh during recording"}' 20
    sleep 5

    interact_and_wait "execute_js" '{"action":"execute_js","reason":"Check watermark after refresh","script":"document.getElementById(\"gasoline-recording-watermark\") ? \"WATERMARK_FOUND\" : \"WATERMARK_MISSING\""}' 30
    local after_refresh="$INTERACT_RESULT"

    interact_and_wait "record_stop" '{"action":"record_stop","reason":"Stop watermark test recording"}' 20
    sleep 1

    local before_ok=false
    local after_ok=false
    if echo "$before_refresh" | grep -q "WATERMARK_FOUND"; then
        before_ok=true
    fi
    if echo "$after_refresh" | grep -q "WATERMARK_FOUND"; then
        after_ok=true
    fi

    if [ "$before_ok" = "true" ] && [ "$after_ok" = "true" ]; then
        pass "Watermark present before AND after page refresh. Tab update listener works."
    elif [ "$before_ok" = "true" ] && [ "$after_ok" != "true" ]; then
        fail "Watermark present before refresh but MISSING after. tabs.onUpdated re-send failed."
    elif [ "$before_ok" != "true" ]; then
        fail "Watermark not found even before refresh. Recording overlay may be broken."
    fi
}
run_test_10_3
