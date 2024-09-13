// Code generated by "libovsdb.modelgen"
// DO NOT EDIT.

package sbdb

import "github.com/ovn-org/libovsdb/model"

const DatapathBindingTable = "Datapath_Binding"

// DatapathBinding defines an object in Datapath_Binding table
type DatapathBinding struct {
	UUID          string            `ovsdb:"_uuid"`
	ExternalIDs   map[string]string `ovsdb:"external_ids"`
	LoadBalancers []string          `ovsdb:"load_balancers"`
	TunnelKey     int               `ovsdb:"tunnel_key"`
}

func (a *DatapathBinding) GetUUID() string {
	return a.UUID
}

func (a *DatapathBinding) GetExternalIDs() map[string]string {
	return a.ExternalIDs
}

func copyDatapathBindingExternalIDs(a map[string]string) map[string]string {
	if a == nil {
		return nil
	}
	b := make(map[string]string, len(a))
	for k, v := range a {
		b[k] = v
	}
	return b
}

func equalDatapathBindingExternalIDs(a, b map[string]string) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if w, ok := b[k]; !ok || v != w {
			return false
		}
	}
	return true
}

func (a *DatapathBinding) GetLoadBalancers() []string {
	return a.LoadBalancers
}

func copyDatapathBindingLoadBalancers(a []string) []string {
	if a == nil {
		return nil
	}
	b := make([]string, len(a))
	copy(b, a)
	return b
}

func equalDatapathBindingLoadBalancers(a, b []string) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if b[i] != v {
			return false
		}
	}
	return true
}

func (a *DatapathBinding) GetTunnelKey() int {
	return a.TunnelKey
}

func (a *DatapathBinding) DeepCopyInto(b *DatapathBinding) {
	*b = *a
	b.ExternalIDs = copyDatapathBindingExternalIDs(a.ExternalIDs)
	b.LoadBalancers = copyDatapathBindingLoadBalancers(a.LoadBalancers)
}

func (a *DatapathBinding) DeepCopy() *DatapathBinding {
	b := new(DatapathBinding)
	a.DeepCopyInto(b)
	return b
}

func (a *DatapathBinding) CloneModelInto(b model.Model) {
	c := b.(*DatapathBinding)
	a.DeepCopyInto(c)
}

func (a *DatapathBinding) CloneModel() model.Model {
	return a.DeepCopy()
}

func (a *DatapathBinding) Equals(b *DatapathBinding) bool {
	return a.UUID == b.UUID &&
		equalDatapathBindingExternalIDs(a.ExternalIDs, b.ExternalIDs) &&
		equalDatapathBindingLoadBalancers(a.LoadBalancers, b.LoadBalancers) &&
		a.TunnelKey == b.TunnelKey
}

func (a *DatapathBinding) EqualsModel(b model.Model) bool {
	c := b.(*DatapathBinding)
	return a.Equals(c)
}

var _ model.CloneableModel = &DatapathBinding{}
var _ model.ComparableModel = &DatapathBinding{}
