package server

import (
	"strings"
	"sync"

	"github.com/golang/glog"
	"github.com/neoul/gnxi/utilities/status"
	"github.com/neoul/gnxi/utilities/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/grpc/codes"
)

type aliasEntry struct {
	name     string
	path     string
	gpath    *gnmipb.Path
	isServer bool // true if it is a client-defined alias
}

type clientAliases struct {
	alias2path       map[string]*aliasEntry
	path2alias       map[string]*aliasEntry
	useServerAliases bool
	mutex            *sync.RWMutex
}

// clientAliases is initialized with server aliases (target-defined aliases)
func newClientAliases() *clientAliases {
	cas := &clientAliases{
		alias2path: make(map[string]*aliasEntry),
		path2alias: make(map[string]*aliasEntry),
		mutex:      &sync.RWMutex{},
	}
	return cas
}

// update updates or deletes the server aliases and returns the updated aliases.
func (cas *clientAliases) UpdateAliases(serverAliases map[string]string, add bool) []string {
	cas.mutex.Lock()
	defer cas.mutex.Unlock()
	aliaslist := make([]string, 0, len(serverAliases))
	for name, path := range serverAliases {
		if !strings.HasPrefix(name, "#") {
			continue
		}
		gpath, err := xpath.ToGNMIPath(path)
		if err != nil {
			continue
		}
		if add {
			if _, ok := cas.alias2path[name]; !ok {
				ca := &aliasEntry{
					name:     name,
					path:     path,
					gpath:    gpath,
					isServer: true,
				}
				cas.path2alias[path] = ca
				cas.alias2path[name] = ca
				aliaslist = append(aliaslist, name)
			}
		} else {
			if ca, ok := cas.alias2path[name]; ok && ca.isServer {
				delete(cas.alias2path, name)
				delete(cas.path2alias, ca.path)
				aliaslist = append(aliaslist, name)
			}
		}
	}
	cas.useServerAliases = add
	return aliaslist
}

// Set sets the client alias to the clientAliases structure.
func (cas *clientAliases) SetAlias(alias *gnmipb.Alias) error {
	cas.mutex.Lock()
	defer cas.mutex.Unlock()
	name := alias.GetAlias()
	if name == "" {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidAlias, "empty alias")
	}
	if !strings.HasPrefix(name, "#") {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidAlias, "alias must start with '#'. e.g. %s", name)
	}
	gpath := alias.GetPath()
	if gpath == nil || len(gpath.GetElem()) == 0 {
		// delete the alias
		if ca, ok := cas.alias2path[name]; ok {
			delete(cas.alias2path, name)
			delete(cas.path2alias, ca.path)
		}
		return nil
	}
	path := xpath.ToXPath(gpath)
	if err := xpath.ValidateGNMIPath(gpath); err != nil {
		return status.TaggedErrorf(codes.InvalidArgument, status.TagInvalidAlias, "invalid path '%s'", path)
	}
	if ca, ok := cas.path2alias[path]; ok {
		return status.TaggedErrorf(codes.AlreadyExists, status.TagInvalidAlias, "'%s'is already defined for '%s'", ca.name, path)
	}
	if ca, ok := cas.alias2path[name]; ok {
		return status.TaggedErrorf(codes.AlreadyExists, status.TagInvalidAlias, "'%s' is already defined for '%s'", name, ca.path)
	}
	// add the alias
	ca := &aliasEntry{
		name:  name,
		path:  path,
		gpath: gpath,
	}
	cas.path2alias[path] = ca
	cas.alias2path[name] = ca
	return nil
}

func (cas *clientAliases) SetAliases(aliases []*gnmipb.Alias) error {
	for _, alias := range aliases {
		if err := cas.SetAlias(alias); err != nil {
			return err
		}
		glog.Infof("Set Alias '%s' to '%s'", alias.GetAlias(), alias.GetPath())
	}
	return nil
}

// ToPath converts an alias to the related path.
// If the input alias is a string, it will return a string path.
// If the input alias is a gnmipb.Path, it will return the same type path.
// if diffFormat is configured, it will return the different type.
//  [gNMI path --> string path, string path --> gNMI path]
func (cas *clientAliases) ToPath(alias interface{}, diffFormat bool) interface{} {
	cas.mutex.RLock()
	defer cas.mutex.RUnlock()
	switch a := alias.(type) {
	case *gnmipb.Path:
		if a == nil {
			if diffFormat {
				return ""
			}
			return a
		}
		for _, e := range a.GetElem() {
			if strings.HasPrefix(e.GetName(), "#") {
				if ca, ok := cas.alias2path[e.GetName()]; ok {
					if diffFormat {
						return ca.path
					}
					return ca.gpath
				}
			}
			break
		}
		if diffFormat {
			return xpath.ToXPath(a)
		}
		return a
	case string:
		if ca, ok := cas.alias2path[a]; ok {
			if diffFormat {
				return ca.gpath
			}
			return ca.path
		}
		if diffFormat {
			o, _ := xpath.ToGNMIPath(a)
			return o
		}
		return a
	}
	// must not reach here!!!
	glog.Fatalf("unknown type inserted to clientAliases.ToPath()")
	return alias
}

// ToAlias converts a path to the related alias.
func (cas *clientAliases) ToAlias(path interface{}, diffFormat bool) interface{} {
	cas.mutex.RLock()
	defer cas.mutex.RUnlock()
	switch path.(type) {
	case *gnmipb.Path:
		gpath := path.(*gnmipb.Path)
		p := xpath.ToXPath(gpath)
		if ca, ok := cas.path2alias[p]; ok {
			if diffFormat {
				return ca.name
			}
			return xpath.GNMIAliasPath(ca.name)
		}
		if diffFormat {
			return p
		}
		return gpath
	case string:
		p := path.(string)
		if ca, ok := cas.path2alias[p]; ok {
			if diffFormat {
				return xpath.GNMIAliasPath(ca.name)
			}
			return ca.name
		}
		if diffFormat {
			o, _ := xpath.ToGNMIPath(p)
			return o
		}
		return p
	}
	// must not reach here!!!
	glog.Fatalf("unknown type inserted to clientAliases.ToAlias()")
	return path
}
