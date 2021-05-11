# dummy-fuse-csi

dummy-fuse implements a dummy FUSE file-system and a CSI Node Plugin that is able to make the file-system available to a workload on a node.

This repository also offers dummy-fuse-workload dummy application that does some I/O periodically (e.g. keeps a file open and reads from it). This is to simulate some useful workload on the FUSE file-system and to trigger various error states.

## Overview

### Motivation

We've seen FUSE-based mounts getting severed in workloads after a CSI Node plugin that provided the mount was restarted, e.g. due to it crashing (for whatever reason) or it being updated (and therefore would need to be restarted). This would manifest in the concerned workload like so:

```
/ # mount | grep fuse
ceph-fuse on /cephfs-share type fuse.ceph-fuse (rw,nosuid,nodev,relatime,user_id=0,group_id=0,allow_other)
/ # ls /cephfs-share
ls: /cephfs-share: Socket not connected
```

or

```
stat /home/kubelet/pods/ae344b80-3b07-4589-b1a1-ca75fa9debf2/volumes/kubernetes.io~csi/pvc-ec69de59-7823-4840-8eee-544f8261fef0/mount: transport endpoint is not connected
```

The reason why this happens is of course because the FUSE file-system driver process lives in the Node plugin container. If this container dies, so does the FUSE driver, along with the mount it provides. This makes all FUSE-based drivers (or any mount provider whose lifetime is tied to the lifetime of its container) unreliable in a CSI environment. DISCLAIMER: tested in Kubernetes only.

The purpose of dummy-fuse-csi is to be able to replicate this (mis)behavior as easily as possible without any unrelated components (i.e. an actual storage system) and to act as a testbed for any possible mitigations or fixes of this problem.

This repository provides a read-only, "hello world"-level FUSE file-system (libfuse3) and a CSI Node Plugin to manage the file-system in a container orchestrator. A Helm chart is included for easy deployment in Kubernetes. Once set up, and after creating a volume and a workload that consumes it, the mount life-time issue can be triggered by simply restarting the Node Plugin and then trying to access the volume from inside the workload.

```
$ kubectl exec -it dummy-fuse-pod -- /bin/sh
/ # stat /mnt
stat: can't stat '/mnt': Socket not connected
```

### Possible mitiagations

#### Mount again

The Node Plugin could try to re-mount the volumes it was providing before it was restarted. dummy-fuse-csi implements this functionality, but unfortunately this isn't enough and works only halfway. The other half of the problem is stale bind-mounts in workloads. When a Pod is being created, Kubernetes will share the respective CSI mounts with the Pod, and this happens only during Pod start-up. When the Node Plugin remounts the respective volumes, Kubernetes would need to share them with the respective Pods again. 

Even if this worked, if the workload/application was storing open file descriptors from the volume before the FUSE process died, all of these would be invalid now anyway.

#### Separate FUSE containers

The FUSE process doesn't necessarily need to live in the same container as the Node Plugin. Should the Node Plugin container die, the FUSE container might survive. Still, if the FUSE driver itself crashes, or needs to be updated, we'd end up in the same situation. Not a real solution.

#### Proper Kubernetes support

The naive safest way to handle this problem would be to monitor for unhealthy volumes (which we can already do). Based on this information, the Pods that make use of the concerned volumes could be restarted. This would trigger volume unmount-mount cycle, effectively restoring the mounts.

## Deployment

A Helm chart provided in `chart/dummy-fuse-csi` may be used to deploy the dummy-fuse-csi Node Plugin in the cluster. For Helm v3 use the following command:

```
helm install <deployment name> chart/dummy-fuse-csi
```

`csi.plugin.restoreMounts` chart value is set to `true` by default. It attempts (but ultimately fails) to restore existing mounts on startup.

### restoreMounts mitigation

When `csi.plugin.restoreMounts` chart value is enabled, dummy-fuse-csi attempts to restore existing mounts on startup.

It stores mount instructions (device and mountpoint paths, mount flags) in node-local storage (a hostPath volume). These instructions are created for each `NodeStageVolume` and `NodePublishVolume` RPC and then later removed on `NodeUnpublishVolume` and `NodeUnstageVolume` RPCs. Should the node plugin be restarted after Stage/Publish calls, but before Unpublish/Unstage calls, dummy-fuse-csi will attempt to replay the stored mount instructions, remounting the volumes.

This only restores FUSE and bind mounts from the node plugin onto the node. This is only one half of the solution. The other is restoring these mounts from the node and into the Pods that consume the concerned volumes. In order to do that, the CO must restart these consumer Pods, or they need to trigger the restart themselves (e.g. when a consumer container dies due to I/O error). Kubernetes doesn't implement this functionality yet, and it depends on the particular application how it handles I/O errors and if such error would make it `exit()`.

All in all, this is a very unreliable mitigation and not a general solution at all.

## Demo

```
$ helm install d chart/dummy-fuse-csi/
NAME: d
LAST DEPLOYED: Fri Apr 16 15:32:20 2021
NAMESPACE: default
STATUS: deployed
REVISION: 1
TEST SUITE: None
```

```
$ kubectl get all
NAME                         READY   STATUS    RESTARTS   AGE
pod/d-dummy-fuse-csi-qqm88   2/2     Running   0          86s
...
```

`manifests` directory contains definitions for:
* a PV/PVC,
* a Pod that mounts the volume, and the application opens a file inside the volume and periodically reads from it.

```
$ kubectl create -f manifests/volume.yaml 
persistentvolume/dummy-fuse-pv created
persistentvolumeclaim/dummy-fuse-pvc created

$ kubectl get pvc
NAME             STATUS   VOLUME          CAPACITY   ACCESS MODES   STORAGECLASS   AGE
dummy-fuse-pvc   Bound    dummy-fuse-pv   1Gi        RWX                           7s

$ kubectl create -f manifests/pod.yaml
pod/dummy-fuse-pod created
```

dummy-fuse-csi Node Plugin logs:
```
$  kubectl logs pod/d-dummy-fuse-csi-qqm88 -c nodeplugin
2021/04/16 13:32:24 Driver: dummy-fuse-csi
2021/04/16 13:32:24 Driver version: 00df51e
2021/04/16 13:32:24 Attempting to re-mount volumes
2021/04/16 13:32:24 No mount cache entries in /csi/mountcache/staged
2021/04/16 13:32:24 No mount cache entries in /csi/mountcache/published
2021/04/16 13:32:24 Registering Identity server
2021/04/16 13:32:24 Registering Node server
2021/04/16 13:32:24 Listening for connections on &net.UnixAddr{Name:"/csi/csi.sock", Net:"unix"}
2021/04/16 13:32:24 [ID:1] GRPC call: /csi.v1.Identity/GetPluginInfo
2021/04/16 13:32:24 [ID:1] GRPC request: {}
2021/04/16 13:32:24 [ID:1] GRPC response: {"name":"dummy-fuse-csi","vendor_version":"00df51e"}
2021/04/16 13:32:25 [ID:2] GRPC call: /csi.v1.Node/NodeGetInfo
2021/04/16 13:32:25 [ID:2] GRPC request: {}
2021/04/16 13:32:25 [ID:2] GRPC response: {"node_id":"rvasek-vanilla-v1-20-df4kxys4fb2w-node-0"}
2021/04/16 13:36:21 [ID:3] GRPC call: /csi.v1.Node/NodeGetCapabilities
2021/04/16 13:36:21 [ID:3] GRPC request: {}
2021/04/16 13:36:21 [ID:3] GRPC response: {"capabilities":[{"Type":{"Rpc":{"type":1}}}]}
2021/04/16 13:36:21 [ID:4] GRPC call: /csi.v1.Node/NodeStageVolume
2021/04/16 13:36:21 [ID:4] GRPC request: {"staging_target_path":"/var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount","volume_capability":{"AccessType":{"Mount":{}},"access_mode":{"mode":5}},"volume_id":"dummy-fuse-volume"}
2021/04/16 13:36:21 exec mountpoint [/var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount]
2021/04/16 13:36:21 exec dummy-fuse [/var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount]
2021/04/16 13:36:22 Saved stage mount entry to /csi/mountcache/staged/dummy-fuse-volume
2021/04/16 13:36:22 [ID:4] GRPC response: {}
2021/04/16 13:36:22 [ID:5] GRPC call: /csi.v1.Node/NodeGetCapabilities
2021/04/16 13:36:22 [ID:5] GRPC request: {}
2021/04/16 13:36:22 [ID:5] GRPC response: {"capabilities":[{"Type":{"Rpc":{"type":1}}}]}
2021/04/16 13:36:22 [ID:6] GRPC call: /csi.v1.Node/NodePublishVolume
2021/04/16 13:36:22 [ID:6] GRPC request: {"staging_target_path":"/var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount","target_path":"/var/lib/kubelet/pods/fa5cb279-460e-441d-9c33-b5ef13702306/volumes/kubernetes.io~csi/dummy-fuse-pv/mount","volume_capability":{"AccessType":{"Mount":{}},"access_mode":{"mode":5}},"volume_id":"dummy-fuse-volume"}
2021/04/16 13:36:22 exec mountpoint [/var/lib/kubelet/pods/fa5cb279-460e-441d-9c33-b5ef13702306/volumes/kubernetes.io~csi/dummy-fuse-pv/mount]
2021/04/16 13:36:22 exec mount [--bind /var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount /var/lib/kubelet/pods/fa5cb279-460e-441d-9c33-b5ef13702306/volumes/kubernetes.io~csi/dummy-fuse-pv/mount]
2021/04/16 13:36:22 Saved publish mount entry to /csi/mountcache/published/dummy-fuse-volume
2021/04/16 13:36:22 [ID:6] GRPC response: {}
```

`dummy-fuse-pod` Pod mounts `dummy-fuse-pvc` PVC into `/mnt`. We can inspect its contents and check the logs:

```
$ kubectl exec dummy-fuse-pod -- /bin/sh -c "mount | grep /mnt ; ls -l /mnt"
dummy-fuse on /mnt type fuse.dummy-fuse (rw,nosuid,nodev,relatime,user_id=0,group_id=0)
total 0
-r--r--r--    1 root     root            13 Jan  1  1970 dummy-file.txt

$ kubectl logs pod/dummy-fuse-pod
2021/04/26 08:47:21 opened file /mnt/dummy-file.txt
2021/04/26 08:47:21 reading
2021/04/26 08:47:26 reading
2021/04/26 08:47:31 reading
```

Everything is fine up until now.

**Trouble ahead:**

```
$ kubectl delete pod/d-dummy-fuse-csi-qqm88
pod "d-dummy-fuse-csi-qqm88" deleted
```

```
$ kubectl logs pod/d-dummy-fuse-csi-5p9jm -c nodeplugin
2021/04/16 13:42:41 Driver: dummy-fuse-csi
2021/04/16 13:42:41 Driver version: 00df51e
2021/04/16 13:42:41 Attempting to re-mount volumes
2021/04/16 13:42:41 exec fusermount3 [-u /var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount]
2021/04/16 13:42:41 exec dummy-fuse [/var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount]
2021/04/16 13:42:41 successfully remounted  /var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount
2021/04/16 13:42:41 exec umount [/var/lib/kubelet/pods/fa5cb279-460e-441d-9c33-b5ef13702306/volumes/kubernetes.io~csi/dummy-fuse-pv/mount]
2021/04/16 13:42:41 exec mount [--bind /var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount /var/lib/kubelet/pods/fa5cb279-460e-441d-9c33-b5ef13702306/volumes/kubernetes.io~csi/dummy-fuse-pv/mount]
2021/04/16 13:42:41 successfully remounted /var/lib/kubelet/pods/fa5cb279-460e-441d-9c33-b5ef13702306/volumes/kubernetes.io~csi/dummy-fuse-pv/mount /var/lib/kubelet/plugins/kubernetes.io/csi/pv/dummy-fuse-pv/globalmount
2021/04/16 13:42:41 Registering Identity server
2021/04/16 13:42:41 Registering Node server
2021/04/16 13:42:41 Listening for connections on &net.UnixAddr{Name:"/csi/csi.sock", Net:"unix"}
2021/04/16 13:42:41 [ID:1] GRPC call: /csi.v1.Identity/GetPluginInfo
2021/04/16 13:42:41 [ID:1] GRPC request: {}
2021/04/16 13:42:41 [ID:1] GRPC response: {"name":"dummy-fuse-csi","vendor_version":"00df51e"}
2021/04/16 13:42:41 [ID:2] GRPC call: /csi.v1.Node/NodeGetInfo
2021/04/16 13:42:41 [ID:2] GRPC request: {}
2021/04/16 13:42:41 [ID:2] GRPC response: {"node_id":"rvasek-vanilla-v1-20-df4kxys4fb2w-node-0"}
```

Inspect `dummy-fuse-pod` again:

```
$ kubectl exec dummy-fuse-pod -- /bin/sh -c "mount | grep /mnt ; ls -l /mnt"
dummy-fuse on /mnt type fuse.dummy-fuse (rw,nosuid,nodev,relatime,user_id=0,group_id=0)
ls: /mnt: Socket not connected
command terminated with exit code 1

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

## Demo with dummy-fuse-csi `csi.plugin.restoureMounts=true` and dummy-fuse-workload `--exit-on-error`

It's possible to restore lost mounts in a very specific setup:
* Deploy dummy-fuse-csi with `helm install d chart/dummy-fuse-csi --set csi.plugin.restoureMounts=true` (enabled by default). See [`restoreMounts` mitigation](#restoremounts-mitigation) for how this is implemented.
* Deploy dummy-fuse-workload with `--exit-on-error` flag. This makes dummy-fuse-workload exit with a non-zero code when it encounters any I/O error.

Now we delete dummy-fuse-csi Node Plugin again:
```
$ kubectl delete pod/d-dummy-fuse-csi-67skb
pod "d-dummy-fuse-csi-67skb" deleted
```

```
$ kubectl logs pod/dummy-fuse-pod -f
2021/04/28 16:32:08 opened file /mnt/dummy-file.txt
2021/04/28 16:32:08 reading
...
2021/04/28 16:32:38 reading
2021/04/28 16:32:38 read error: read /mnt/dummy-file.txt: transport endpoint is not connected
(EOF)
```

Only this time `dummy-fuse-pod` Pod is back up!
```
$ kubectl get pods
NAME                                                 READY   STATUS    RESTARTS   AGE
dummy-fuse-pod                                       1/1     Running   1          6m44s

$ kubectl describe pod/dummy-fuse-pod
...

Events:
  Type     Reason     Age                From               Message
  ----     ------     ----               ----               -------
  Normal   Scheduled  71s                default-scheduler  Successfully assigned default/dummy-fuse-pod to rvasek-testing-1-20-fson6szf6fed-node-0
  Warning  Failed     40s                kubelet            Error: failed to generate container "e98a54c3969ef4cedab8f6f4b58a8ecc1b40a07ad514afb6eb388cd45d12e952" spec: failed to generate spec: failed to stat "/var/lib/kubelet/pods/8af8cf09-b2a1-4648-abf2-9e7f9677bf84/volumes/kubernetes.io~csi/dummy-fuse-pv/mount": stat /var/lib/kubelet/pods/8af8cf09-b2a1-4648-abf2-9e7f9677bf84/volumes/kubernetes.io~csi/dummy-fuse-pv/mount: transport endpoint is not connected
  Normal   Pulled     26s (x3 over 70s)  kubelet            Container image "rvasek/dummy-fuse-csi:latest" already present on machine
  Normal   Created    25s (x2 over 70s)  kubelet            Created container dummy-workload
  Normal   Started    25s (x2 over 70s)  kubelet            Started container dummy-workload

$ kubectl logs pod/dummy-fuse-pod
2021/04/28 16:32:53 opened file /mnt/dummy-file.txt
2021/04/28 16:32:53 reading
2021/04/28 16:32:58 reading
2021/04/28 16:33:03 reading
...
```

This demo shows that it's possible to recover from broken FUSE mounts without any changes to Kubernetes, if:
* the consumer application exits on at least `ENOTCONN`,
* the CSI driver must be able to restore mounts on restart. This is not always possible, because in order to store the "mount instructions", some drivers would need to store also volume credentials.
