# dummy-fuse-csi

dummy-fuse-csi implements a dummy FUSE file-system and a CSI Node Plugin that is able to make the file-system available to a workload on the node.

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

The purpose of dummy-fuse-csi is to be able to replicate this (mis)behavior as easily as possible without any unrelated components (i.e. an actual storage system) and to act as a testbed for any possible mitiagations or fixes of this problem.

dummy-fuse-csi provides a "hello world"-level FUSE file-system (libfuse3) and a CSI Node Plugin to manage the file-system in a container orchestrator. A Helm chart is included for easy deployment in Kubernetes. Once set up, and after creating a volume and a workload that consumes it, the mount life-time issue can be triggered by simply restarting the Node Plugin and then trying to access the volume from inside the workload.

```
$ kubectl exec -it dummy-fuse-pod -- /bin/sh
/ # stat /mnt
stat: can't stat '/mnt': Socket not connected
```

### Possible mitiagations

#### Mount again

The Node Plugin could try to re-mount the volumes it was providing before it was restarted. dummy-fuse-csi implements this functionality, but unfortunately this isn't enough and works only halfway. The other half of the problem is stale bind-mounts in workloads. When a Pod is being created, Kubernetes will share the respective CSI mounts with the Pod, and this happends only during Pod start-up. When the Node Plugin remounts the respective volumes, Kubernetes would need to share them with the respective Pods again. 

Even if this worked, if the workload/application was storing open file descriptors from the volume before the FUSE process died, all of these would be invalid now anyway.

#### Separate FUSE containers

The FUSE process doesn't necessarily need to live in the same container as the Node Plugin. Should the Node Plugin container die, the FUSE container might survive. Still, the FUSE driver itself crashes, or needs to be updated, we'd end up in the same situation.

#### Proper Kubernetes support

The naive safest way to handle this problem would be to monitor for unhealthy volumes (which we can already do). Based on this information, the Node Plugin along with all the Pods that make use of the volumes it provides could be restarted. All volumes would then be mounted again, effectively solving the problem.

This is of course only one of the possible ways, and needs to be discussed with sig-storage.

## Deployment

A Helm chart provided in `chart/dummy-fuse-csi` may be used to deploy the dummy-fuse-csi Node Plugin in the cluster. For Helm v3 use the following command:

```
helm install <deployment name> chart/dummy-fuse-csi
```

`csi.plugin.restoreMounts` chart value is set to `true` by default. It attempts (but ultimately fails) to restore existing mounts on startup.

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
