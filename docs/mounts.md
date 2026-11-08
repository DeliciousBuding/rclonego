# Mount cache and offline files

RcloneGo mounts use rclone's `full` VFS disk cache. Put the cache on a local SSD
to improve repeated reads and random access. Its configured size and age are
eviction targets; an open or pending-write file can keep the cache above its
size target. The cache is not an independent backup or a guaranteed offline copy.

Cache status is read from the official
[`vfs/stats` and `vfs/queue` RC endpoints](https://rclone.org/rc/#vfs-stats).
RcloneGo contacts only a loopback endpoint discovered on a mount whose PID and
process start identity match this instance's recorded owner. Existing mounts
owned by another process remain usable, but their cache status is unavailable
until ownership is explicitly established.

The status includes:

- Disk cache bytes, cached files, errored files and out-of-space state.
- Uploads queued and uploads in progress.
- In-memory directory and file metadata counts.
- Up to 200 queued upload details, including size, retry count and eligibility
  time. The response marks omitted queue details and unavailable samples.

`vfs_reference_count` is the number of references to the VFS instance. It does
not count open files. An upload's `expiry` is the number of seconds until it is
eligible to start and may be negative; it is not an estimate of completion time.
The official responses do not provide a cache-hit ratio.

Mounted writes can be accepted locally before they reach cloud storage. Check
the queued, uploading and errored-file state before treating a change as uploaded.
Status is a sample and can change immediately after it is read.
Disk usage follows rclone's cache accounting and can lag a recent file read.

## Persistent offline copies

An offline copy adds an ordinary `copy` or `copyto` job to the shared queue.
Choose a named remote source and an absolute local destination directory outside
rclone's internal VFS cache. A file source is saved under its basename in that
directory; a directory source copies its contents there.

```json
{
  "name": "Offline documents",
  "source": "example-remote:documents",
  "destination": "/path/to/offline/documents",
  "file": false,
  "idempotency_key": "offline-documents-1"
}
```

Use `"file": true` for a single file, such as
`"source": "example-remote:archive/vault.bin"`. The destination still names a
directory.

The local result can be opened without the cloud mount after the job completes.
Copy preserves destination-only files, but it can replace matching destination
files according to rclone's comparison rules. It does not continuously sync the
offline copy back to the remote. Transfer completion and content verification
are separate; use a content-check workflow when complete byte verification is
required.

A large encrypted container can make many random requests when opened through a
cloud mount. An SSD cache helps repeated access; a full offline copy removes the
network from local reads. Neither changes the application's decryption cost or
makes the first uncached remote read instantaneous.

## Mount logs

The recommended rclone engine rotates each mount log at 8 MiB, retains at most
two older compressed logs and expires them after 30 days. Rotation uses rclone's
official log flags, so a mount can continue logging after the desktop process
exits. Cache files and queued uploads are never removed by log retention.

## 缓存与离线副本

挂载使用 rclone 的完整 VFS 缓存，建议放在本机 SSD。缓存会按容量和保留时间
淘汰，正在使用或等待写回的文件可能使它暂时超过上限。缓存不等于备份，也不
保证永久离线可用。

缓存状态来自官方 VFS 接口，只读取本实例拥有且进程启动身份匹配的挂载。
界面可显示缓存容量、文件数、待写回、正在上传、失败文件和空间不足状态。
引用计数不是打开文件数，上传资格时间也不是预计完成时间。

离线副本会进入同一个任务队列，将远端目录或文件复制到用户选择的本机目录。
该目录必须在 VFS 内部缓存之外。任务完成后可以直接打开本机文件；它不会自动
双向同步，也不会删除目标独有文件。大加密容器首次云端随机读取仍然受网络
延迟影响，完整下载到 SSD 后再打开能避免这部分等待。
