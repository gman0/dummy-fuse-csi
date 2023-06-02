dummy-fuse-workload opens a file descriptor and periodically reads from the fd.

Usage: `dummy-fuse-workload -file FILEPATH [-exit-on-error] [-read-interval 5]`

After restarting dummy-fuse-csi Pod, dummy-fuse-workload logs following error:

```
$ kubectl logs pod/dummy-fuse-pod
2021/04/26 08:47:21 opened file /mnt/dummy-file.txt
...
2021/04/26 08:48:36 reading
2021/04/26 08:48:36 read error: read /mnt/dummy-file.txt: transport endpoint is not connected
2021/04/26 08:48:41 reading
2021/04/26 08:48:41 read error: read /mnt/dummy-file.txt: transport endpoint is not connected
2021/04/26 08:48:46 reading
2021/04/26 08:48:46 read error: read /mnt/dummy-file.txt: transport endpoint is not connected
```

This is because the file descriptor is no longer valid once the FUSE mount is broken.
