package reconciler

import (
	"context"
	"fmt"
	"log"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Opts struct {
	NodeName           string
	CSIDriverName      string
	ReconciliationTime time.Time
}

type objRef struct {
	name      string
	namespace string
}

func (o objRef) String() string {
	return fmt.Sprintf("%s/%s", o.namespace, o.name)
}

func Run(ctx context.Context, c kubernetes.Interface, o *Opts) error {
	podsOnNode, err := c.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("spec.nodeName=%s", o.NodeName),
	})
	if err != nil {
		return fmt.Errorf("failed to LIST Pods: %v", err)
	}

	podNames := func() []string {
		xs := make([]string, len(podsOnNode.Items))
		for i := range podsOnNode.Items {
			p := podsOnNode.Items[i]
			xs[i] = fmt.Sprintf("%s/%s", p.Namespace, p.Name)
		}
		return xs
	}()

	log.Printf("Found %d Pods on node %s: %v\n", len(podsOnNode.Items), o.NodeName, podNames)

	podsForDeletion, err := selectPodsForDeletion(ctx, c, o, podsOnNode.Items)
	if err != nil {
		return fmt.Errorf("failed to select Pods for deletion: %v", err)
	}

	log.Printf("Selected %d Pods for deletion: %v\n", len(podsForDeletion), podsForDeletion)

	for _, podRef := range podsForDeletion {
		if err := c.CoreV1().Pods(podRef.namespace).Delete(ctx, podRef.name, metav1.DeleteOptions{}); err != nil {
			return fmt.Errorf("failed to DELETE Pod %s: %v", podRef.name, err)
		}
	}

	return nil
}

// Select only pods that:
// * are scheduled on the specified node,
// * are not static,
// * have been created before Reconcilier's pod,
// * are created from a PersistentVolumeClaimVolumeSource
//   and are managed by CSI driver specified in the Opts.
func selectPodsForDeletion(
	ctx context.Context,
	c kubernetes.Interface,
	o *Opts,
	pods []corev1.Pod,
) ([]objRef, error) {
	var podsForDeletion []objRef
	volumesManagedByCSIDriver := make(map[string]bool)

	for _, p := range pods {
		if p.Spec.NodeName != o.NodeName {
			log.Printf("Pod %s/%s is not scheduled on node %s, skipping\n", p.Namespace, p.Name, o.NodeName)
			continue
		}
		if isStaticPod(&p) {
			log.Printf("Pod %s/%s is not managed by a DaemonSet/StatefulSet/ReplicaSet, skipping\n", p.Namespace, p.Name)
			continue
		}
		if !p.CreationTimestamp.Time.Before(o.ReconciliationTime) {
			log.Printf("Pod %s/%s was created after %v, skipping\n", p.Namespace, p.Name, o.ReconciliationTime)
			continue
		}
		if res, err := hasCSIManagedVolume(ctx, c, &p, o, volumesManagedByCSIDriver); err != nil {
			return nil, err
		} else if !res {
			log.Printf("Pod %s/%s doesn't have volumes managed by %s, skipping\n", p.Namespace, p.Name, o.CSIDriverName)
			continue
		}

		podsForDeletion = append(podsForDeletion, objRef{
			name:      p.Name,
			namespace: p.Namespace,
		})
	}

	return podsForDeletion, nil
}

func isStaticPod(p *corev1.Pod) bool {
	ret := true

refsLoop:
	for _, o := range p.GetOwnerReferences() {
		switch o.Kind {
		case "DaemonSet", "StatefulSet", "ReplicaSet":
			ret = false
			break refsLoop
		}
	}

	return ret
}

func hasCSIManagedVolume(
	ctx context.Context,
	c kubernetes.Interface,
	p *corev1.Pod,
	o *Opts,
	csiManagedVolumes map[string]bool,
) (bool, error) {
	for _, v := range p.Spec.Volumes {
		// TODO: We're considering only PVC volume sources for now.
		// CSIVolumeSource and EphemeralVolumeSource?

		if v.PersistentVolumeClaim == nil {
			continue
		}

		s := v.PersistentVolumeClaim

		if isManaged, ok := csiManagedVolumes[s.ClaimName]; ok {
			// Result is cached in csiManagedVolumes.
			// It's sufficient to find at least one volume managed by the
			// specified CSI driver in order for the Pod to be eligible
			// for deletion.
			if isManaged {
				return true, nil
			}
		} else {
			// Not found. Query for PVC's PV, find out if it's managed by
			// this CSI driver and store the result in `csiManagedVolumes`
			// cache.

			pvc, err := c.CoreV1().PersistentVolumeClaims(p.Namespace).Get(ctx, s.ClaimName, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to GET PersistentVolumeClaim %s/%s: %v", p.Namespace, s.ClaimName, err)
			}

			if pvc.Spec.VolumeName == "" {
				continue
			}

			pv, err := c.CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
			if err != nil {
				return false, fmt.Errorf("failed to GET PersistentVolume %s: %v", pvc.Spec.VolumeName, err)
			}

			isManaged = pv.Spec.CSI != nil && pv.Spec.CSI.Driver == o.CSIDriverName
			csiManagedVolumes[pv.Name] = isManaged

			if isManaged {
				return true, nil
			}
		}
	}

	return false, nil
}
