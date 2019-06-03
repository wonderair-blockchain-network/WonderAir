// Copyright (c) 2018 The wonderair ecosystem Authors
// Distributed under the MIT software license, see the accompanying
// file COPYING or or or http://www.opensource.org/licenses/mit-license.php


// +build linux darwin freebsd

package fuse

import (
	"os"
	"path/filepath"
	"sync"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"golang.org/x/net/context"
)

var (
	_ fs.Node                = (*SwarmDir)(nil)
	_ fs.NodeRequestLookuper = (*SwarmDir)(nil)
	_ fs.HandleReadDirAller  = (*SwarmDir)(nil)
	_ fs.NodeCreater         = (*SwarmDir)(nil)
	_ fs.NodeRemover         = (*SwarmDir)(nil)
	_ fs.NodeMkdirer         = (*SwarmDir)(nil)
)

type SwarmDir struct {
	inode       uint64
	name        string
	path        string
	directories []*SwarmDir
	files       []*SwarmFile

	mountInfo *MountInfo
	lock      *sync.RWMutex
}

func NewSwarmDir(fullpath string, minfo *MountInfo) *SwarmDir {
	newdir := &SwarmDir{
		inode:       NewInode(),
		name:        filepath.Base(fullpath),
		path:        fullpath,
		directories: []*SwarmDir{},
		files:       []*SwarmFile{},
		mountInfo:   minfo,
		lock:        &sync.RWMutex{},
	}
	return newdir
}

func (sd *SwarmDir) Attr(ctx context.Context, a *fuse.Attr) error {
	a.Inode = sd.inode
	a.Mode = os.ModeDir | 0700
	a.Uid = uint32(os.Getuid())
	a.Gid = uint32(os.Getegid())
	return nil
}

func (sd *SwarmDir) Lookup(ctx context.Context, req *fuse.LookupRequest, resp *fuse.LookupResponse) (fs.Node, error) {

	for _, n := range sd.files {
		if n.name == req.Name {
			return n, nil
		}
	}
	for _, n := range sd.directories {
		if n.name == req.Name {
			return n, nil
		}
	}
	return nil, fuse.ENOENT
}

func (sd *SwarmDir) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	var children []fuse.Dirent
	for _, file := range sd.files {
		children = append(children, fuse.Dirent{Inode: file.inode, Type: fuse.DT_File, Name: file.name})
	}
	for _, dir := range sd.directories {
		children = append(children, fuse.Dirent{Inode: dir.inode, Type: fuse.DT_Dir, Name: dir.name})
	}
	return children, nil
}

func (sd *SwarmDir) Create(ctx context.Context, req *fuse.CreateRequest, resp *fuse.CreateResponse) (fs.Node, fs.Handle, error) {

	newFile := NewSwarmFile(sd.path, req.Name, sd.mountInfo)
	newFile.fileSize = 0 // 0 means, file is not in swarm yet and it is just created

	sd.lock.Lock()
	defer sd.lock.Unlock()
	sd.files = append(sd.files, newFile)

	return newFile, newFile, nil
}

func (sd *SwarmDir) Remove(ctx context.Context, req *fuse.RemoveRequest) error {

	if req.Dir && sd.directories != nil {
		newDirs := []*SwarmDir{}
		for _, dir := range sd.directories {
			if dir.name == req.Name {
				removeDirectoryFromSwarm(dir)
			} else {
				newDirs = append(newDirs, dir)
			}
		}
		if len(sd.directories) > len(newDirs) {
			sd.lock.Lock()
			defer sd.lock.Unlock()
			sd.directories = newDirs
		}
		return nil
	} else if !req.Dir && sd.files != nil {
		newFiles := []*SwarmFile{}
		for _, f := range sd.files {
			if f.name == req.Name {
				removeFileFromSwarm(f)
			} else {
				newFiles = append(newFiles, f)
			}
		}
		if len(sd.files) > len(newFiles) {
			sd.lock.Lock()
			defer sd.lock.Unlock()
			sd.files = newFiles
		}
		return nil
	}
	return fuse.ENOENT
}

func (sd *SwarmDir) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {

	newDir := NewSwarmDir(req.Name, sd.mountInfo)

	sd.lock.Lock()
	defer sd.lock.Unlock()
	sd.directories = append(sd.directories, newDir)

	return newDir, nil

}
