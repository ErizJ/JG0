package engine

import (
	"strings"
)

// 添加两个方法, 存入通配符规则 和 通过uri获取路径
type treeNode struct {
	name       string
	children   []*treeNode
	routerName string
	isEnd      bool
}

var sep = "/"

func (t *treeNode) Put(path string, routerName ...string) {
	//每次只能构建一个链路
	pathArr := strings.Split(path, sep)
	rName := ""
	if len(routerName) > 0 {
		rName = routerName[0]
	}
	pathLen := len(pathArr)
	for i := 1; i < pathLen; i++ {
		name := pathArr[i]
		children := t.children
		isMatch := false
		for _, child := range children {
			if child.name == name {
				isMatch = true
				t = child
				break
			}
		}
		if !isMatch {
			isEnd := i == pathLen-1
			node := &treeNode{name: name, children: make([]*treeNode, 0), isEnd: isEnd, routerName: rName}
			children = append(children, node)
			t.children = children
			t = node
		}
	}
}

func (t *treeNode) Get(path string) *treeNode {
	pathArr := strings.Split(path, sep)
	routerName := ""
	for index, name := range pathArr {
		if index == 0 {
			continue
		}
		children := t.children
		isMatch := false
		for _, child := range children {
			if child.name == name || child.name == "*" || strings.Contains(child.name, ":") {
				isMatch = true
				routerName += sep + child.name
				if child.routerName == "" {
					child.routerName = routerName
				}
				t = child
				if index == len(pathArr)-1 {
					return child
				}
				break
			}
		}
		if !isMatch {
			for _, child := range children {
				if child.name == "**" {
					routerName += "/" + name
					if child.routerName == "" {
						child.routerName = routerName
					}
					return child
				}
			}
		}
	}
	return nil
}
