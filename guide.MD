# 🧩 Distributed File Storage System

## 📌 Requirements

A collection of independent machines should be able to register with a central server and **store portions of uploaded files**. Here's how the system should behave:

- If there are **N machines** registered, each file uploaded will be **split into N chunks**.
- Each machine will store **1/N-th** of the file's data.
- The **server** is responsible for:
  - Managing the **order of chunks**
  - Tracking which **machines hold which chunks**
- When a client requests a file, the server will:
  - **Reassemble** it from the distributed chunks
  - Return the complete file to the client

---

## 🛠️ Development Plan

### ✅ Step 1: Machine Registration
- [x] Machines should be able to **register their IPs** with the server.
- [x] The IP list should be **persisted to disk** for server restarts.
- [x] Maintain a mapping of **IP addresses to names** to help identify machines when distributing files.

### ⚙️ Step 2: File Upload & Distribution
- [ ] Server accepts a file and **splits it across X machines**.
- [ ] Upload returns a **fileID** for future retrieval.
- [ ] The number of splits does **not** have to match the exact number of machines (can be between 1 and N).
- [ ] Maintain a mapping:  
  `HashMap[fileID] => HashMap[machineIP] => fileChunk`
- [ ] Keep the **splitting logic and server selection algorithm loosely coupled**, ideally abstracted behind an interface for flexibility.

### 📥 Step 3: File Retrieval
- [ ] Clients can retrieve their full file by passing the **fileID**.
- [ ] The reconstructed file should be **byte-for-byte identical** to the original.
- [ ] Retrieve each file chunk from the appropriate machines using the `fileID -> machine -> chunk` map.
- [ ] Reassemble the chunks **in order**, then return to the client.
- [ ] (Optional/Future) Consider whether to **remove file chunks** after successful reconstruction.

### 🔁 Step 4: Graceful Server Restart
- [x] Server should handle shutdowns without data loss.
- [x] All critical state must be **persisted regularly**:
  - Machine registration data
  - File-to-machine mappings
  - Chunk-to-machine mappings
- [x] Ensure reloading of state from disk on startup.

---

## 📂 Future Considerations

- Authenticated client uploads/downloads
- Replication and redundancy
- Fault tolerance & machine failure handling
- Chunk encryption

~~~~~~~~~~~~~~~~~~~~~




---

## ✅ Implemented Server Commands

The server currently supports three basic commands over a TCP connection, all sent as a single line via terminal using `netcat (nc)`.
use -e with echo command to simulate new line (\n)
---

### 1. **Register for File Sharing**
**Command:**
```
store:
```

**Example:**
```bash
(echo "store:"; echo "hello from client") | nc localhost 9999
```

---

### 2. **Client Registration (Debug Mode)**
**Command:**
```
:c
```

**Example:**
```bash
(echo ":c"; echo "hello client") | nc localhost 9999
```

---

### 3. **Shutdown Server**
**Command:**
```
EndServer:4444
```

**Example:**
```bash
echo -e "EndServer:4444" | nc localhost 9999
```

This triggers a shutdown signal handled directly by the server.

---
