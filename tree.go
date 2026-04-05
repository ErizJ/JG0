package msgo

import (
	"strings"
)

type treeNode struct {
	name       string
	children   []*treeNode
	routerName string
	isEnd      bool
}

//添加两个方法, 存入通配符规则 和 通过uri获取路径

func (t *treeNode) Put(path string) {
	//每次只能构建一个链路
	pathArr := strings.Split(path, "/")
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
			node := &treeNode{name: name, children: make([]*treeNode, 0), isEnd: isEnd}
			children = append(children, node)
			t.children = children
			t = node
		}
	}
}

func (t *treeNode) Get(path string) *treeNode {
	pathArr := strings.Split(path, "/")
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
				routerName += "/" + child.name
				child.routerName = routerName
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
					child.routerName = routerName
					return child
				}
			}
		}
	}
	return nil
}
