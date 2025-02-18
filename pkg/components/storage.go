package components

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/klog/v2"

	"github.com/openshift/microshift/pkg/assets"
	"github.com/openshift/microshift/pkg/config"
	"github.com/openshift/microshift/pkg/config/lvmd"
)

// getCSIPluginConfig searches for a user-defined lvmd configuration file in /etc/microshift.  If found, returns
// the lvmd config.  If not found, returns a default-value lvmd config.  If an unmarshalling errors, returns nil
// and the error.
func getCSIPluginConfig() (*lvmd.Lvmd, error) {
	lvmdConfig := filepath.Join(filepath.Dir(config.DefaultGlobalConfigFile), lvmd.LvmdConfigFileName)
	if _, err := os.Stat(lvmdConfig); !errors.Is(err, os.ErrNotExist) {
		return lvmd.NewLvmdConfigFromFile(lvmdConfig)
	}
	return (&lvmd.Lvmd{}).WithDefaults(), nil
}

func startCSIPlugin(cfg *config.MicroshiftConfig, kubeconfigPath string) error {
	var (
		ns = []string{
			"components/lvms/topolvm-openshift-storage_namespace.yaml",
		}
		sa = []string{
			"components/lvms/topolvm-node_v1_serviceaccount.yaml",
			"components/lvms/topolvm-controller_v1_serviceaccount.yaml",
		}
		role = []string{
			"components/lvms/topolvm-controller_rbac.authorization.k8s.io_v1_role.yaml",
			"components/lvms/topolvm-csi-provisioner_rbac.authorization.k8s.io_v1_role.yaml",
			"components/lvms/topolvm-csi-resizer_rbac.authorization.k8s.io_v1_role.yaml",
		}
		rb = []string{
			"components/lvms/topolvm-controller_rbac.authorization.k8s.io_v1_rolebinding.yaml",
			"components/lvms/topolvm-csi-provisioner_rbac.authorization.k8s.io_v1_rolebinding.yaml",
			"components/lvms/topolvm-csi-resizer_rbac.authorization.k8s.io_v1_rolebinding.yaml",
		}
		cr = []string{
			"components/lvms/topolvm-csi-provisioner_rbac.authorization.k8s.io_v1_clusterrole.yaml",
			"components/lvms/topolvm-controller_rbac.authorization.k8s.io_v1_clusterrole.yaml",
			"components/lvms/topolvm-csi-resizer_rbac.authorization.k8s.io_v1_clusterrole.yaml",
			"components/lvms/topolvm-node-scc_rbac.authorization.k8s.io_v1_clusterrole.yaml",
			"components/lvms/topolvm-node_rbac.authorization.k8s.io_v1_clusterrole.yaml",
		}
		crb = []string{
			"components/lvms/topolvm-controller_rbac.authorization.k8s.io_v1_clusterrolebinding.yaml",
			"components/lvms/topolvm-csi-provisioner_rbac.authorization.k8s.io_v1_clusterrolebinding.yaml",
			"components/lvms/topolvm-csi-resizer_rbac.authorization.k8s.io_v1_clusterrolebinding.yaml",
			"components/lvms/topolvm-node-scc_rbac.authorization.k8s.io_v1_clusterrolebinding.yaml",
			"components/lvms/topolvm-node_rbac.authorization.k8s.io_v1_clusterrolebinding.yaml",
		}
		cd = []string{
			"components/lvms/csi-driver.yaml",
		}
		cm = []string{
			"components/lvms/topolvm-lvmd-config_configmap_v1.yaml",
		}
		ds = []string{
			"components/lvms/topolvm-node_daemonset.yaml",
		}
		deploy = []string{
			"components/lvms/topolvm-controller_deployment.yaml",
		}
		sc = []string{
			"components/lvms/topolvm_default-storage-class.yaml",
		}
		scc = []string{
			"components/lvms/topolvm-node-securitycontextconstraint.yaml",
		}
	)

	// the lvmd file should be located in the same directory as the microshift config to minimize coupling with the
	// csi plugin.
	lvmdCfg, err := getCSIPluginConfig()
	if err != nil {
		return err
	}
	lvmdRenderParams, err := renderLvmdParams(lvmdCfg)
	if err != nil {
		return fmt.Errorf("rendering lvmd params: %v", err)
	}

	if err := assets.ApplyStorageClasses(sc, nil, nil, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply storage cass %v: %v", sc, err)
		return err
	}
	if err := assets.ApplyCSIDrivers(cd, nil, nil, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply csiDriver %v: %v", sc, err)
		return err
	}
	if err := assets.ApplyNamespaces(ns, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply ns %v: %v", ns, err)
		return err
	}
	if err := assets.ApplyServiceAccounts(sa, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply sa %v: %v", sa, err)
		return err
	}
	if err := assets.ApplyRoles(role, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply role %v: %v", cr, err)
		return err
	}
	if err := assets.ApplyRoleBindings(rb, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply rolebinding %v: %v", cr, err)
		return err
	}
	if err := assets.ApplyClusterRoles(cr, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply clusterrole %v: %v", cr, err)
		return err
	}
	if err := assets.ApplyClusterRoleBindings(crb, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply clusterrolebinding %v: %v", crb, err)
		return err
	}
	if err := assets.ApplyConfigMaps(cm, renderTemplate, lvmdRenderParams, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply configMap %v: %v", crb, err)
		return err
	}
	if err := assets.ApplyDeployments(deploy, renderTemplate, renderParamsFromConfig(cfg, nil), kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply deployment %v: %v", deploy, err)
		return err
	}
	if err := assets.ApplyDaemonSets(ds, renderTemplate, renderParamsFromConfig(cfg, lvmdRenderParams), kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply daemonsets %v: %v", ds, err)
		return err
	}
	if err := assets.ApplySCCs(scc, nil, nil, kubeconfigPath); err != nil {
		klog.Warningf("Failed to apply sccs %v: %v", scc, err)
		return err
	}
	return nil
}
