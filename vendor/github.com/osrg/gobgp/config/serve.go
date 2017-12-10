package config

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"os/signal"
	"syscall"
)

type BgpConfigSet struct {
	Global            Global             `mapstructure:"global"`
	Neighbors         []Neighbor         `mapstructure:"neighbors"`
	PeerGroups        []PeerGroup        `mapstructure:"peer-groups"`
	RpkiServers       []RpkiServer       `mapstructure:"rpki-servers"`
	BmpServers        []BmpServer        `mapstructure:"bmp-servers"`
	MrtDump           []Mrt              `mapstructure:"mrt-dump"`
	Zebra             Zebra              `mapstructure:"zebra"`
	Collector         Collector          `mapstructure:"collector"`
	DefinedSets       DefinedSets        `mapstructure:"defined-sets"`
	PolicyDefinitions []PolicyDefinition `mapstructure:"policy-definitions"`
	DynamicNeighbors  []DynamicNeighbor  `mapstructure:"dynamic-neighbors"`
}

func ReadConfigfileServe(path, format string, configCh chan *BgpConfigSet) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP)

	// Update config file type, if detectable
	format = detectConfigFileType(path, format)

	cnt := 0
	for {
		c := &BgpConfigSet{}
		v := viper.New()
		v.SetConfigFile(path)
		v.SetConfigType(format)
		var err error
		if err = v.ReadInConfig(); err != nil {
			goto ERROR
		}
		if err = v.UnmarshalExact(c); err != nil {
			goto ERROR
		}
		if err = setDefaultConfigValuesWithViper(v, c); err != nil {
			goto ERROR
		}
		if cnt == 0 {
			log.WithFields(log.Fields{
				"Topic": "Config",
			}).Info("Finished reading the config file")
		}
		cnt++
		configCh <- c
		goto NEXT
	ERROR:
		if cnt == 0 {
			log.WithFields(log.Fields{
				"Topic": "Config",
				"Error": err,
			}).Fatalf("Can't read config file %s", path)
		} else {
			log.WithFields(log.Fields{
				"Topic": "Config",
				"Error": err,
			}).Warningf("Can't read config file %s", path)
		}
	NEXT:
		select {
		case <-sigCh:
			log.WithFields(log.Fields{
				"Topic": "Config",
			}).Info("Reload the config file")
		}
	}
}

func inSlice(n Neighbor, b []Neighbor) int {
	for i, nb := range b {
		if nb.State.NeighborAddress == n.State.NeighborAddress {
			return i
		}
	}
	return -1
}

func existPeerGroup(n string, b []PeerGroup) int {
	for i, nb := range b {
		if nb.Config.PeerGroupName == n {
			return i
		}
	}
	return -1
}

func ConfigSetToRoutingPolicy(c *BgpConfigSet) *RoutingPolicy {
	return &RoutingPolicy{
		DefinedSets:       c.DefinedSets,
		PolicyDefinitions: c.PolicyDefinitions,
	}
}

func UpdatePeerGroupConfig(curC, newC *BgpConfigSet) ([]PeerGroup, []PeerGroup, []PeerGroup) {
	addedPg := []PeerGroup{}
	deletedPg := []PeerGroup{}
	updatedPg := []PeerGroup{}

	for _, n := range newC.PeerGroups {
		if idx := existPeerGroup(n.Config.PeerGroupName, curC.PeerGroups); idx < 0 {
			addedPg = append(addedPg, n)
		} else if !n.Equal(&curC.PeerGroups[idx]) {
			log.WithFields(log.Fields{
				"Topic": "Config",
			}).Debugf("Current peer-group config:%s", curC.PeerGroups[idx])
			log.WithFields(log.Fields{
				"Topic": "Config",
			}).Debugf("New peer-group config:%s", n)
			updatedPg = append(updatedPg, n)
		}
	}

	for _, n := range curC.PeerGroups {
		if existPeerGroup(n.Config.PeerGroupName, newC.PeerGroups) < 0 {
			deletedPg = append(deletedPg, n)
		}
	}
	return addedPg, deletedPg, updatedPg
}

func UpdateNeighborConfig(curC, newC *BgpConfigSet) ([]Neighbor, []Neighbor, []Neighbor) {
	added := []Neighbor{}
	deleted := []Neighbor{}
	updated := []Neighbor{}

	for _, n := range newC.Neighbors {
		if idx := inSlice(n, curC.Neighbors); idx < 0 {
			added = append(added, n)
		} else if !n.Equal(&curC.Neighbors[idx]) {
			log.WithFields(log.Fields{
				"Topic": "Config",
			}).Debugf("Current neighbor config:%s", curC.Neighbors[idx])
			log.WithFields(log.Fields{
				"Topic": "Config",
			}).Debugf("New neighbor config:%s", n)
			updated = append(updated, n)
		}
	}

	for _, n := range curC.Neighbors {
		if inSlice(n, newC.Neighbors) < 0 {
			deleted = append(deleted, n)
		}
	}
	return added, deleted, updated
}

func CheckPolicyDifference(currentPolicy *RoutingPolicy, newPolicy *RoutingPolicy) bool {

	log.WithFields(log.Fields{
		"Topic": "Config",
	}).Debugf("Current policy:%s", currentPolicy)
	log.WithFields(log.Fields{
		"Topic": "Config",
	}).Debugf("New policy:%s", newPolicy)

	var result bool = false
	if currentPolicy == nil && newPolicy == nil {

		result = false
	} else {
		if currentPolicy != nil && newPolicy != nil {
			result = !currentPolicy.Equal(newPolicy)
		} else {
			result = true
		}
	}
	return result
}
