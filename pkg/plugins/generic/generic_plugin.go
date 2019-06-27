package main

import (
	"reflect"

	"github.com/golang/glog"
	sriovnetworkv1 "github.com/openshift/sriov-network-operator/pkg/apis/sriovnetwork/v1"
	"github.com/openshift/sriov-network-operator/pkg/utils"
)

type GenericPlugin struct {
	PluginName  string
	SpecVersion string
	DesireState *sriovnetworkv1.SriovNetworkNodeState
	LastState   *sriovnetworkv1.SriovNetworkNodeState
}

var Plugin GenericPlugin

// Initialize our plugin and set up initial values
func init() {
	Plugin = GenericPlugin{
		PluginName:  "generic_plugin",
		SpecVersion: "1.0",
	}
}

// Name returns the name of the plugin
func (p *GenericPlugin) Name() string {
	return p.PluginName
}

// SpecVersion returns the version of the spec expected by the plugin
func (p *GenericPlugin) Spec() string {
	return p.SpecVersion
}

// OnNodeStateAdd Invoked when SriovNetworkNodeState CR is created, return if need dain and/or reboot node
func (p *GenericPlugin) OnNodeStateAdd(state *sriovnetworkv1.SriovNetworkNodeState) (needDrain bool, needReboot bool, err error) {
	glog.Info("generic-plugin OnNodeStateAdd()")
	needDrain = false
	needReboot = false
	err = nil

	if len(state.Spec.Interfaces) > 0 {
		needDrain = true
	}

	p.DesireState = state
	return
}

// OnNodeStateChange Invoked when SriovNetworkNodeState CR is updated, return if need dain and/or reboot node
func (p *GenericPlugin) OnNodeStateChange(old, new *sriovnetworkv1.SriovNetworkNodeState) (needDrain bool, needReboot bool, err error) {
	glog.Info("generic-plugin OnNodeStateChange()")
	needDrain = false
	needReboot = false
	p.DesireState = new
	if new.Spec.DpConfigVersion != old.Spec.DpConfigVersion && (len(new.Spec.Interfaces) > 0 || len(old.Spec.Interfaces) > 0) {
		glog.Infof("generic-plugin OnNodeStateChange(): CMRV changed %v -> %v", old.Spec.DpConfigVersion, new.Spec.DpConfigVersion)
		needDrain = true
		return
	}

	err = nil
	found := false
	for _, in := range new.Spec.Interfaces {
		found = false
		for _, io := range old.Spec.Interfaces {
			if in.PciAddress == io.PciAddress {
				found = true
				if in.NumVfs != io.NumVfs {
					needDrain = true
				}
			}
		}
		if !found {
			needDrain = true
		}
	}
	return
}

// Apply config change
func (p *GenericPlugin) Apply() error {
	glog.Infof("generic-plugin Apply(): desiredState=%v", p.DesireState.Spec)
	if p.LastState != nil {
		glog.Infof("generic-plugin Apply(): lastStat=%v", p.LastState.Spec)
		if reflect.DeepEqual(p.LastState.Spec.Interfaces, p.DesireState.Spec.Interfaces) {
			glog.Info("generic-plugin Apply(): nothing to apply")
			return nil
		}
	}
	exit, err := utils.Chroot("/host")
	if err != nil {
		return err
	}
	if err := utils.SyncNodeState(p.DesireState); err != nil {
		return err
	}
	if err := exit(); err != nil {
		return err
	}
	p.LastState = &sriovnetworkv1.SriovNetworkNodeState{}
	*p.LastState = *p.DesireState
	return nil
}
