# File Upload via Go Server — Tech Spec

**Feature:** Automated file upload capability for Gasoline extension, using the Go server's filesystem access to bypass browser sandbox restrictions.

**Use Case:** Bulk video uploads to platforms without APIs (e.g., Rumble, YouTube, etc.)

---

## Problem Statement

Chrome extensions run in a sandboxed environment that prevents:
- Reading arbitrary files from disk
- Programmatically populating `<input type="file">` elements
- Direct filesystem access from JavaScript

However, Gasoline has a Go server component running as a native process that **can** read files from disk. This spec defines how to leverage that capability.

---

## Activation: Explicit User Opt-In

This feature requires **explicit user consent via startup flag**. It is NOT enabled by default.

**Why:**
- File upload automation is a workaround, not standard functionality
- User should be fully aware they're enabling non-standard behavior
- Prevents accidental activation
- Easier to audit and disable

**Startup:**
```bash
gasoline-mcp --enable-upload-automation
# or
GASOLINE_UPLOAD_AUTOMATION=true gasoline-mcp
```

**Without this flag:**
- Upload-related endpoints return 403 Forbidden
- Extension UI does not show upload options
- All upload functionality is disabled

This ensures users must be deliberate about enabling workaround behavior.

---

## Architecture

### Current Gasoline Architecture
```
Browser Extension (sandboxed)
  ↓ stdio/HTTP
Go Server (native process, unrestricted filesystem access)
```

### Proposed Flow

```
1. User calls: interact(action: "upload", selector: "#Filedata", file_path: "/path/to/video.mp4")
2. Extension sends file_path to Go server
3. Go server reads file from disk → encodes to base64
4. Go server sends file data back to extension
5. Extension uses execute_js to:
   a. Create File/Blob object from base64 data
   b. Inject into file input element (via extension privileges)
   c. Trigger change event
   d. Optionally submit form
```

---

## Implementation Details

### 1. New Tool Action: `interact(action: "upload", ...)`

**Parameters:**
- `selector` (required): CSS selector for file input element
- `file_path` (required): Absolute path to file on user's disk
- `submit` (optional, default: false): Whether to auto-submit form after upload

**Example:**
```typescript
await interact({
  action: "upload",
  selector: "#Filedata",
  file_path: "/Users/brenn/Videos/my-video.mp4",
  submit: true
});
```

**Response:**
```json
{
  "success": true,
  "file_size_bytes": 1024000000,
  "file_name": "my-video.mp4",
  "input_element": {
    "id": "Filedata",
    "type": "file",
    "accept": "video/*"
  }
}
```

### 2. Go Server Implementation

**New endpoint:** `POST /api/file/read`

**Request:**
```json
{
  "file_path": "/Users/brenn/Videos/my-video.mp4"
}
```

**Response:**
```json
{
  "success": true,
  "file_name": "my-video.mp4",
  "file_size": 1024000000,
  "mime_type": "video/mp4",
  "data_base64": "AAAA...[base64 encoded file content]...AAAA"
}
```

**Error Handling:**
- File not found → `{"error": "file_not_found", "path": "..."}` (404)
- Permission denied → `{"error": "permission_denied", "path": "..."}` (403)
- File too large (>15GB) → `{"error": "file_too_large", "size": ...}` (413)

### 3. Extension Implementation

**Flow in content script:**

```typescript
// 1. Read file from Go server
const response = await fetch('http://localhost:9223/api/file/read', {
  method: 'POST',
  body: JSON.stringify({ file_path: '/path/to/video.mp4' })
});
const fileData = await response.json();

// 2. Decode base64 to Blob
const binaryString = atob(fileData.data_base64);
const bytes = new Uint8Array(binaryString.length);
for (let i = 0; i < binaryString.length; i++) {
  bytes[i] = binaryString.charCodeAt(i);
}
const blob = new Blob([bytes], { type: fileData.mime_type });

// 3. Create File object
const file = new File([blob], fileData.file_name, { type: fileData.mime_type });

// 4. Inject into file input
const fileInput = document.querySelector('#Filedata');
// Use extension's elevated privileges to create a FileList
// This is where extension privileges come in—regular scripts can't do this
```

**Key Challenge:** Creating a `FileList` object and assigning it to `HTMLInputElement.files`

**Solution Options:**

#### Option A: Drag-Drop Simulation (Most Compatible)
```typescript
const dataTransfer = new DataTransfer();
dataTransfer.items.add(file);

const dropEvent = new DragEvent('drop', {
  bubbles: true,
  cancelable: true,
  dataTransfer
});

fileInput.dispatchEvent(dropEvent);
```

**Pros:** Works in most upload handlers
**Cons:** Requires that the upload handler listens to drop events

#### Option B: Extension Content Script Privileges
Use `chrome.scripting.executeScript()` with `world: "MAIN"` to run in page context with elevated privileges:

```typescript
chrome.scripting.executeScript({
  target: { tabId },
  function: injectFile,
  args: [file, fileInputSelector]
});

function injectFile(file, selector) {
  const input = document.querySelector(selector);
  // In MAIN world, may have access to internal APIs
  // But still blocked by browser security model
}
```

**Pros:** Highest privilege level for extension
**Cons:** Still subject to browser security restrictions on file inputs

#### Option C: Direct API Submission (Platform-Specific)
If the platform supports it, bypass the file input entirely:

```typescript
const formData = new FormData();
formData.append('file', file);
formData.append('title', 'My Video');

const uploadResponse = await fetch('https://platform.com/api/upload', {
  method: 'POST',
  body: formData
});
```

**Pros:** No sandbox issues
**Cons:** Requires knowing the platform's API endpoint; may not work for Rumble

---

## Rumble-Specific Implementation

### Discovery Results

**File Input:** `#Filedata`
- **Accept:** `video/mp4,video/x-m4v,video/*`
- **Name:** `Filedata`
- **Form Action:** `https://rumble.com/upload.php`

**Form Fields:**
- `#title` — Video Title (required)
- `#category_primary` — Primary Category
- `#category_secondary` — Secondary Category
- `#tags` — Tags (comma-separated, optional)
- Various visibility/scheduling/monetization options

**Upload Method:** Drag-drop supported ("or drag and drop the file here")

**Max File Size:** 15 GB (stated in UI)

### Workflow for Rumble Bulk Upload

1. Read spreadsheet with columns: `file_path`, `title`, `description`, `tags`, `category`, ...
2. For each row:
   ```
   a. interact(fill, selector="#title", text="From spreadsheet")
   b. interact(fill, selector="#tags", text="tag1,tag2")
   c. interact(upload, selector="#Filedata", file_path="/path/from/spreadsheet")
   d. interact(click, selector="#submitForm")
   e. wait for upload to complete
   ```

---

## State Machine

### Upload States

```
IDLE
  ↓ [user calls interact(action="upload")]
READING_FILE
  ↓ [Go server reads from disk]
FILE_LOADED
  ↓ [extension receives base64 data]
INJECTING_FILE
  ↓ [extension injects into DOM]
READY_FOR_SUBMISSION
  ↓ [user calls click(selector="#submitForm") or auto-submit]
UPLOADING
  ↓ [upload progress via network observer]
COMPLETE
  ↓ [success or error]
IDLE
```

---

## Edge Cases & Recovery

| Scenario | Root Cause | Recovery |
|----------|-----------|----------|
| **File not found** | User provided invalid path | Return 404 error, suggest checking path |
| **File too large (>15GB)** | Platform limit | Return 413 error, suggest splitting files |
| **Permission denied** | File protected by OS | Return 403 error, suggest checking permissions |
| **Extension privileges insufficient** | Browser sandbox blocks File API manipulation | Fall back to drag-drop simulation |
| **Drag-drop doesn't work** | Platform doesn't support drop events | Document as limitation; suggest alternative approach |
| **Network timeout during upload** | Large file, slow connection | Implement progress polling with configurable timeout |
| **Form not found** | Selector mismatch or page changed | Return error with debugging info (page screenshot) |
| **Multiple files selected accidentally** | Platform doesn't support multiple | Reject with error message |

---

## Network Communication

### File Read Request (Extension → Go Server)

```
POST /api/file/read HTTP/1.1
Host: localhost:9223
Content-Type: application/json

{
  "file_path": "/Users/brenn/Videos/rumble-upload.mp4"
}
```

### File Read Response

```
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: [size]

{
  "success": true,
  "file_name": "rumble-upload.mp4",
  "file_size": 1073741824,
  "mime_type": "video/mp4",
  "data_base64": "AAAA...truncated for brevity..."
}
```

### Error Responses

```
HTTP/1.1 404 Not Found
{
  "error": "file_not_found",
  "file_path": "/Users/brenn/Videos/missing.mp4",
  "message": "File does not exist"
}

HTTP/1.1 413 Payload Too Large
{
  "error": "file_too_large",
  "file_size": 16000000000,
  "max_size": 15000000000,
  "message": "File exceeds 15GB platform limit"
}
```

---

## Performance Considerations

### Base64 Encoding Overhead
- Base64 increases file size by ~33%
- 1GB file → 1.33GB in memory
- **Risk:** Memory exhaustion on large files

**Mitigation:** Implement streaming for files >2GB

### Race Conditions
- If user clicks file input manually while Go server is still reading → conflict
- **Mitigation:** Disable/hide file input during read operation

### Concurrent Uploads
- Multiple `interact(action="upload")` calls simultaneously
- **Mitigation:** Queue file reads on Go server; one at a time

---

## Deployment & Testing

### Go Server Changes
1. Add `/api/file/read` endpoint
2. Implement file validation (size, permissions)
3. Add error handling for all edge cases
4. Test with various file sizes and paths

### Extension Changes
1. Extend `interact` tool with `upload` action
2. Implement file injection logic (test both drag-drop and direct injection)
3. Add progress tracking for large files
4. Test with Rumble upload form (live)

### UAT Scenarios
1. ✅ Small file (< 100MB) upload
2. ✅ Large file (> 1GB) upload
3. ✅ File not found (verify error handling)
4. ✅ Permission denied (verify error handling)
5. ✅ Concurrent uploads (verify queuing)
6. ✅ Form field population + file upload (full workflow)

---

## Browser Sandbox Constraints

### What Works
- ✅ Content script can read from Go server via HTTP
- ✅ Extension has elevated privileges (can use more APIs)
- ✅ Can dispatch events to DOM elements
- ✅ Can create Blob/File objects from data

### What's Blocked
- ❌ Cannot directly set `HTMLInputElement.files` (read-only)
- ❌ Cannot read arbitrary local files directly from extension
- ❌ Cannot programmatically trigger native file picker
- ❌ Cannot fake file inputs with synthetic drag-drop events
- ❌ Platforms reject drag-drop with synthetic File objects (security)

### Reality Check: Rumble Testing Results

**Tested Drag-Drop on Rumble:** ❌ FAILED

- Form uses custom JavaScript handler (not native form submission)
- No Dropzone.js or Uppy detected
- Synthetic drag-drop events silently ignored by Rumble's handler
- **Root Cause:** Rumble only accepts files from real `<input type="file">` elements (browser security measure)

### Base64 Encoding Limitations

For multi-GB videos, base64 approach is **NOT VIABLE**:

| File Size | Base64 Overhead | Memory Peak | Status |
|-----------|-----------------|------------|--------|
| 1 GB | +33% (1.33 GB) | 2.66 GB | Possible |
| 4 GB | +33% (5.3 GB) | 10.6 GB | Problematic |
| 10 GB | +33% (13.3 GB) | 26.6 GB | ❌ Crashes |

Additional issues:
- JavaScript string size limits (~2GB practical maximum)
- CPU overhead of encoding/decoding
- Network transmission inefficiency (5.3GB over stdio)
- No streaming progress reporting

### Workarounds Employed
- Use Go server for filesystem access (outside sandbox) ✅
- Use drag-drop simulation (browser allows, but platforms reject) ❌
- Pass file data as Blob/File (works for small files only) ⚠️

---

## Revised Approach: Intelligent Fallback Strategy

Instead of choosing one method, Gasoline should **auto-detect and use the least invasive approach**:

### Stage 1: Synthetic Drag-Drop (Least Invasive) ✅
```
Try: Synthetic drag-drop with File objects
If works: Use it (no server overhead)
If fails: Escalate to Stage 2
```

### Stage 2: File Dialog Interception (Medium)
```
Try: CDP file dialog auto-response
If works: User clicks file input, CDP responds with path
If fails: Escalate to Stage 3
```

### Stage 3: Form Submission Monitoring (Most Invasive)
```
Try: Intercept actual form submission, extract CSRF/cookies
Replicate: POST from Go server with user's auth credentials
If works: Fully automated
If fails: Suggest hybrid approach
```

---

## Stage 3 Implementation: Form Interception

### Auto-Detection Algorithm

**1. Detect Form & Extract Fields**
```javascript
// In CDP-controlled browser
const form = document.querySelector('form');
const fields = {};

// Get all form fields
form.querySelectorAll('input, textarea, select').forEach(el => {
  if (el.name) fields[el.name] = el.value;
});

// Detect CSRF token (try common names)
const csrfNames = ['csrf_token', '_token', '__RequestVerificationToken', 'authenticity_token'];
let csrfField = null;
for (const name of csrfNames) {
  if (fields[name]) {
    csrfField = name;
    break;
  }
}

return {
  action: form.action,
  method: form.method,
  enctype: form.enctype,
  fields: fields,
  csrf_field: csrfField,
  csrf_value: fields[csrfField],
  file_input_name: form.querySelector('input[type="file"]')?.name
};
```

**2. Extract Session Cookies**
```javascript
// Get all cookies (accessible to page scripts)
const cookies = document.cookie
  .split(';')
  .map(c => c.trim())
  .join('; ');

// Also check for auth tokens in sessionStorage/localStorage
const authData = {
  cookies: cookies,
  sessionStorage: JSON.parse(JSON.stringify(sessionStorage)),
  localStorage: JSON.parse(JSON.stringify(localStorage))
};
```

**3. Monitor Actual Submission**
```javascript
// Hook the form to capture submission details
const originalAction = form.action;
form.onsubmit = async (e) => {
  e.preventDefault();

  const formData = new FormData(form);
  const submission = {
    action: form.action,
    method: form.method,
    enctype: form.enctype,
    fields: Object.fromEntries(formData),
    timestamp: Date.now(),
    // Critical for replay
    csrf_token: formData.get(csrfField),
    cookies: document.cookie,
    headers: {
      'Referer': window.location.href,
      'Origin': window.location.origin,
      'User-Agent': navigator.userAgent
    }
  };

  // Send to Go server for replay
  await fetch('http://localhost:9223/api/upload-with-file', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(submission)
  });
};
```

### Go Server: Smart Form Replay

```go
type FormSubmission struct {
  Action    string            `json:"action"`
  Method    string            `json:"method"`
  Enctype   string            `json:"enctype"`
  Fields    map[string]string `json:"fields"`
  CSRFToken string            `json:"csrf_token"`
  Cookies   string            `json:"cookies"`
  Headers   map[string]string `json:"headers"`
}

func uploadWithFormReplay(submission FormSubmission, filePath string) error {
  // 1. Validate we have auth
  if submission.Cookies == "" {
    return fmt.Errorf("no session cookies; user must be logged in")
  }

  // 2. Build multipart form
  body := &bytes.Buffer{}
  writer := multipart.NewWriter(body)

  // Add all form fields
  for k, v := range submission.Fields {
    if k != "Filedata" { // Skip the file input itself
      writer.WriteField(k, v)
    }
  }

  // Add the file
  file, err := os.Open(filePath)
  if err != nil {
    return fmt.Errorf("file not found: %s", filePath)
  }
  defer file.Close()

  fileWriter, _ := writer.CreateFormFile("Filedata", filepath.Base(filePath))
  io.Copy(fileWriter, file)
  writer.Close()

  // 3. Build request with exact headers
  req, _ := http.NewRequest(submission.Method, submission.Action, body)

  // Set content type
  req.Header.Set("Content-Type", writer.FormDataContentType())

  // Set session cookies (critical!)
  req.Header.Set("Cookie", submission.Cookies)

  // Set security headers from original submission
  for k, v := range submission.Headers {
    req.Header.Set(k, v)
  }

  // 4. Execute upload
  resp, err := http.DefaultClient.Do(req)
  if err != nil {
    return fmt.Errorf("upload failed: %w", err)
  }
  defer resp.Body.Close()

  // 5. Check response
  if resp.StatusCode >= 400 {
    body, _ := io.ReadAll(resp.Body)
    return fmt.Errorf("upload rejected (HTTP %d): %s", resp.StatusCode, string(body))
  }

  return nil
}
```

### State Machine: Intelligent Escalation

```
IDLE
  ↓ [User calls interact(action: "upload", ...)]
STAGE_1_DRAGDROP
  ├─ [Success] → COMPLETE ✅
  └─ [Fails] → STAGE_2_FILE_DIALOG

STAGE_2_FILE_DIALOG
  ├─ [Success] → COMPLETE ✅
  └─ [Fails] → STAGE_3_FORM_INTERCEPTION

STAGE_3_FORM_INTERCEPTION
  ├─ [No CSRF] → ERROR (require manual upload)
  ├─ [No cookies] → ERROR (user must be logged in)
  ├─ [Success] → COMPLETE ✅
  └─ [CSRF mismatch] → ERROR (form structure changed)

COMPLETE
  ↓ [Return success + method used]
```

### Error Recovery

| Scenario | Detection | Recovery |
|----------|-----------|----------|
| CSRF token changed | Token mismatch between stages | Re-extract token, retry |
| Session expired | 401/403 response | Suggest user re-login |
| Form structure changed | Missing expected fields | Document and suggest manual upload |
| File too large | 413 response | Fall back to chunked upload |
| Network timeout | Connection error | Retry with exponential backoff |

---

## Platform-Agnostic Design

This approach works for **any upload form** that:
- ✅ Uses standard HTML forms
- ✅ Has CSRF protection (auto-detected)
- ✅ Requires user auth (cookies detected)
- ✅ Accepts multipart/form-data POSTs

Examples: YouTube, Vimeo, Dailymotion, custom platforms, etc.

---

## Alternative: Explicit DevTools Permission Model

**Security-First Approach:** Instead of hidden automation, require user to grant explicit DevTools access.

### Setup Flow

```
1. User installs Gasoline extension
2. Gasoline prompts: "Enable bulk upload mode?"
3. User clicks "Grant DevTools Access"
4. Browser shows: chrome://inspect → User grants Gasoline permission
5. User provides Rumble login (or uses existing session)
6. User provides spreadsheet CSV with: file_path, title, tags, etc.
```

### Workflow (With Explicit Permission)

```
Extension detects form submission:
├─ User is logged into Rumble
├─ User loads upload page
├─ Gasoline injects capture hook:
│  window.gasoline_uploadMode = true;
│
├─ User clicks "Start Bulk Upload" (explicit action)
├─ Form submit is intercepted
├─ Captured: action, method, fields, cookies, CSRF token
│
└─ Sent to Go server:
   POST /api/upload-bulk
   {
     "videos": [
       {"file_path": "/path/video1.mp4", "title": "Video 1", ...},
       {"file_path": "/path/video2.mp4", "title": "Video 2", ...}
     ],
     "form_action": "https://rumble.com/upload.php",
     "cookies": "session=xyz;...",
     "csrf_token": "abc123"
   }
```

### Go Server Handler

```go
// /api/upload-bulk
func handleBulkUpload(ctx context.Context, req BulkUploadRequest) error {
  for i, video := range req.Videos {
    file, err := os.Open(video.FilePath)
    if err != nil {
      return fmt.Errorf("video %d: file not found: %w", i, err)
    }
    defer file.Close()

    // Build multipart form
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)

    // Add form fields
    for k, v := range req.FormFields {
      writer.WriteField(k, v)
    }

    // Add metadata from spreadsheet
    writer.WriteField("title", video.Title)
    writer.WriteField("tags", video.Tags)
    writer.WriteField("category_primary", video.Category)

    // Add file
    fw, _ := writer.CreateFormFile("Filedata", filepath.Base(video.FilePath))
    io.Copy(fw, file)
    writer.Close()

    // POST with user's session
    httpReq, _ := http.NewRequest("POST", req.FormAction, body)
    httpReq.Header.Set("Cookie", req.Cookies)
    httpReq.Header.Set("Referer", "https://rumble.com/upload.php")
    httpReq.Header.Set("Content-Type", writer.FormDataContentType())

    resp, err := http.DefaultClient.Do(httpReq)
    if err != nil {
      return fmt.Errorf("video %d: upload failed: %w", i, err)
    }
    resp.Body.Close()

    if resp.StatusCode >= 400 {
      return fmt.Errorf("video %d: rejected (HTTP %d)", i, resp.StatusCode)
    }

    // Wait between uploads to avoid rate limiting
    time.Sleep(5 * time.Second)
  }

  return nil
}
```

### Comparison: Hidden vs. Explicit

| Aspect | Hidden Automation | Explicit Permission |
|--------|-------------------|-------------------|
| **User Experience** | "Magic happens" | Transparent, user-controlled |
| **Security Posture** | Implicit trust | Explicit consent |
| **Debugging** | Hard to troubleshoot | User can watch/verify |
| **Compliance** | Harder to justify | Clear audit trail |
| **Implementation** | CDP auto-responders | Simple hook + POST |
| **Error Recovery** | Complex state machine | Direct feedback to user |
| **Trust** | Feels sneaky | Feels deliberate |
| **Code Complexity** | High (auto-detect stages) | Low (~200 lines) |

### When to Use Each

**Hidden Automation (Stage 1-3):**
- Power users who want fully hands-off
- Bulk uploads of 100+ files
- Running on servers/scheduled jobs
- Advanced use cases

**Explicit Permission (DevTools):**
- First-time users (clearer what's happening)
- Security-conscious users (transparent process)
- Troubleshooting (user can monitor)
- Learning/testing uploads
- Legal/compliance requirements

---

### Implementation Advantages (Explicit Model)

1. **No complex state machine** — Just capture → POST
2. **Better errors** — User sees exactly what failed
3. **Resume capability** — Stop/restart mid-batch
4. **Audit trail** — Extension logged every step
5. **Easier testing** — Manual testing mirrors automation
6. **User confidence** — They control the process

---

## Alternative Approaches NOT Pursued

1. **File System Access API**
   - Requires user to grant permission for each file (defeats automation)
   - Not suitable for bulk uploads

2. **Native Messaging (for desktop app)**
   - Adds dependency on separate native app
   - Adds complexity; existing Go server solves this

3. **Platform-Specific APIs (YouTube Data API, etc.)**
   - Each platform different; not generalizable
   - Some platforms (Rumble) have no API
   - This approach is platform-agnostic

---

## References

- [Rumble Upload Page](https://rumble.com/upload.php)
- [Chrome Extension File Access](https://developer.chrome.com/docs/extensions/reference/api/scripting/)
- [DataTransfer API](https://developer.mozilla.org/en-US/docs/Web/API/DataTransfer)
- [File API](https://developer.mozilla.org/en-US/docs/Web/API/File)
