package sriovnetworknodepolicy

import (
	"context"
	"fmt"
	"encoding/json"
	"os"

	sriovnetworkv1 "github.com/pliurh/sriov-network-operator/pkg/apis/sriovnetwork/v1"
	render "github.com/pliurh/sriov-network-operator/pkg/render"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	errs "github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var log = logf.Log.WithName("controller_sriovnetworknodepolicy")

// ManifestPaths is the path to the manifest templates
// bad, but there's no way to pass configuration to the reconciler right now
const (
	ManifestPath = "./bindata"
	NAMESPACE = "sriov-network-operator"
	DEFAULT_POLICY_NAME = "default"
)
/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new SriovNetworkNodePolicy Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileSriovNetworkNodePolicy{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("sriovnetworknodepolicy-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource SriovNetworkNodePolicy
	err = c.Watch(&source.Kind{Type: &sriovnetworkv1.SriovNetworkNodePolicy{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner SriovNetworkNodePolicy
	err = c.Watch(&source.Kind{Type: &appsv1.DaemonSet{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &sriovnetworkv1.SriovNetworkNodePolicy{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileSriovNetworkNodePolicy{}

// ReconcileSriovNetworkNodePolicy reconciles a SriovNetworkNodePolicy object
type ReconcileSriovNetworkNodePolicy struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a SriovNetworkNodePolicy object and makes changes based on the state read
// and what is in the SriovNetworkNodePolicy.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSriovNetworkNodePolicy) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling SriovNetworkNodePolicy")

	// Fetch the SriovNetworkNodePolicy instances
	policyList := &sriovnetworkv1.SriovNetworkNodePolicyList{}
	err := r.client.List(context.TODO(), &client.ListOptions{}, policyList)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}
	defaultPolicy := &sriovnetworkv1.SriovNetworkNodePolicy{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: DEFAULT_POLICY_NAME, Namespace: NAMESPACE,}, defaultPolicy)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("Creating a default SriovNetworkNodePolicy to lunch the SR-IoV Daemons")
			defaultPolicy.Namespace = NAMESPACE
			defaultPolicy.Name = DEFAULT_POLICY_NAME
			err = r.client.Create(context.TODO(), defaultPolicy)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// Render new DaemonSet objects
	objs, err := renderObjsForCR()
	if err != nil {
		reqLogger.Error(err, "Failed to render SR-IoV manifests")
		return reconcile.Result{}, err
	}

	for _, obj := range objs {
		err = r.syncObject(defaultPolicy, policyList,obj)
		if err != nil {
			reqLogger.Error(err, "Couldn't sync SR-IoV objects")
			return reconcile.Result{}, err
		}
	}

	// All was successful. Request that this be re-triggered after ResyncPeriod,
	// so we can reconcile state again.
	return reconcile.Result{}, nil
}

func (r *ReconcileSriovNetworkNodePolicy)syncObject(d *sriovnetworkv1.SriovNetworkNodePolicy, l *sriovnetworkv1.SriovNetworkNodePolicyList, obj *uns.Unstructured) error {
	var err error
	logger := log.WithName("syncObjects")
	logger.Info("Start to sync Objects")
	scheme := kscheme.Scheme
	switch kind := obj.GetKind(); kind {
	case "Namespace":
		ns := &corev1.Namespace{}
		err = scheme.Convert(obj, ns, nil)
		r.syncNamespace(d, ns)
		if err != nil {
			logger.Error(err, "Fail to sync Namespace")
			return err
		}
	case "ServiceAccount":
		sa := &corev1.ServiceAccount{}
		err = scheme.Convert(obj, sa, nil)
		r.syncServiceAccount(d, sa)
		if err != nil {
			logger.Error(err, "Fail to sync ServiceAccount")
			return err
		}
	case "DaemonSet":
		ds := &appsv1.DaemonSet{}
		err = scheme.Convert(obj, ds, nil)
		r.syncDaemonSet(d, l, ds)
		if err != nil {
			logger.Error(err, "Fail to sync DaemonSet", "Namespace", ds.Namespace, "Name", ds.Name)
			return err
		}
	}
	return nil
}

func (r *ReconcileSriovNetworkNodePolicy)syncServiceAccount(cr *sriovnetworkv1.SriovNetworkNodePolicy, in *corev1.ServiceAccount) error{
	logger := log.WithName("syncServiceAccount")
	logger.Info("Start to sync ServiceAccount", "Name", in.Name)

	if err := controllerutil.SetControllerReference(cr, in, r.scheme); err != nil {
		return err
	}
	sa := &corev1.ServiceAccount{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: in.Namespace, Name: in.Name}, sa)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.client.Create(context.TODO(), in)
			if err != nil {
				return fmt.Errorf("Couldn't create ServiceAccount: %v", err)
			}
			logger.Info("Created ServiceAccount for", in.Namespace, in.Name)
		} else {
			return fmt.Errorf("Failed to get ServiceAccount: %v", err)
		}
	} else {
		logger.Info("ServiceAccount already exists, updating")
		err = r.client.Update(context.TODO(), in)
		if err != nil {
			return fmt.Errorf("Couldn't update ServiceAccount: %v", err)
		}
	}
	return nil
}

func (r *ReconcileSriovNetworkNodePolicy)syncNamespace(cr *sriovnetworkv1.SriovNetworkNodePolicy, in *corev1.Namespace) error{
	logger := log.WithName("syncNamespace")
	logger.Info("Start to sync Namespaces", "Name", in.Name)

	if err := controllerutil.SetControllerReference(cr, in, r.scheme); err != nil {
		return err
	}
	ns := &corev1.Namespace{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Namespace: in.Namespace, Name: in.Name}, ns)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.client.Create(context.TODO(), in)
			if err != nil {
				return fmt.Errorf("Couldn't create Namespace: %v", err)
			}
			logger.Info("Created Namespace for", in.Namespace, in.Name)
		} else {
			return fmt.Errorf("Failed to get Namespace: %v", err)
		}
	} else {
		logger.Info("Namespace already exists, updating")
		err = r.client.Update(context.TODO(), in)
		if err != nil {
			return fmt.Errorf("Couldn't update Namespace: %v", err)
		}
	}
	return nil
}

func (r *ReconcileSriovNetworkNodePolicy)syncDaemonSet(cr *sriovnetworkv1.SriovNetworkNodePolicy, l *sriovnetworkv1.SriovNetworkNodePolicyList, in *appsv1.DaemonSet) error{
	logger := log.WithName("syncDaemonSet")
	logger.Info("Start to sync DaemonSet", "Name", in.Name)
	var err error

	if err = setDsNodeAffinity(l, in); err != nil {
		return err
	}
	if err = controllerutil.SetControllerReference(cr, in, r.scheme); err != nil {
		return err
	}
	ds := &appsv1.DaemonSet{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Namespace: in.Namespace, Name: in.Name}, ds)
	if err != nil {
		if errors.IsNotFound(err) {
			err = r.client.Create(context.TODO(), in)
			if err != nil {
				return fmt.Errorf("Couldn't create DaemonSet: %v", err)
			}
			logger.Info("Created DaemonSet for", in.Namespace, in.Name)
		} else {
			return fmt.Errorf("Failed to get DaemonSet: %v", err)
		}
	} else {
		logger.Info("DaemonSet already exists, updating")
		err = r.client.Update(context.TODO(), in)
		if err != nil {
			return fmt.Errorf("Couldn't update DaemonSet: %v", err)
		}
	}
	return nil
}

func setDsNodeAffinity(pl *sriovnetworkv1.SriovNetworkNodePolicyList, ds *appsv1.DaemonSet) error {
	terms := ds.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	
	for _, p := range pl.Items {
		nodeSelector := corev1.NodeSelectorTerm{}
		for k, v := range p.Spec.NodeSelector {
			expressions := []corev1.NodeSelectorRequirement{}
			exp :=  corev1.NodeSelectorRequirement{
				Operator: corev1.NodeSelectorOpIn,
				Key: k,
				Values: []string{v},
			}
			expressions = append(expressions, exp)
			nodeSelector = corev1.NodeSelectorTerm{
				MatchExpressions: expressions,
			}
		}
		terms = append(terms, nodeSelector)
	}
	return nil
}

// renderDsForCR returns a busybox pod with the same name/namespace as the cr
func renderObjsForCR() ([]*uns.Unstructured, error) {
	var err error
	objs := []*uns.Unstructured{}

	// render RawCNIConfig manifests
	data := render.MakeRenderData()
	data.Data["SRIOVCNIImage"] = os.Getenv("SRIOV_CNI_IMAGE")
	data.Data["SRIOVDevicePluginImage"] = os.Getenv("SRIOV_DEVICE_PLUGIN_IMAGE")
	data.Data["ReleaseVersion"] = os.Getenv("RELEASEVERSION")
	objs, err = render.RenderDir(ManifestPath, &data)
	if err != nil {
		return nil,errs.Wrap(err, "failed to render OpenShiftSRIOV Network manifests")
	}
	for _, obj := range objs {
		raw, _:= json.Marshal(obj)
		fmt.Printf("manifest %s\n", raw)
	}
	return objs, nil
}
