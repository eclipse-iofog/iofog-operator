/*
 *  *******************************************************************************
 *  * Copyright (c) 2019 Edgeworx, Inc.
 *  *
 *  * This program and the accompanying materials are made available under the
 *  * terms of the Eclipse Public License v. 2.0 which is available at
 *  * http://www.eclipse.org/legal/epl-2.0
 *  *
 *  * SPDX-License-Identifier: EPL-2.0
 *  *******************************************************************************
 *
 */

package util

import "fmt"

// Set by linker
var (
	repo = "undefined"

	controllerTag  = "undefined"
	kubeletTag     = "undefined"
	routerTag      = "undefined"
	portManagerTag = "undefined"
	proxyTag       = "undefined"
)

const (
	controllerImage  = "controller"
	kubeletImage     = "kubelet"
	portManagerImage = "port-manager"
	proxyImage       = "proxy"
	routerImage      = "router"
)

func GetControllerImage() string { return fmt.Sprintf("%s/%s:%s", repo, controllerImage, controllerTag) }
func GetKubeletImage() string    { return fmt.Sprintf("%s/%s:%s", repo, kubeletImage, kubeletTag) }
func GetRouterImage() string     { return fmt.Sprintf("%s/%s:%s", repo, routerImage, routerTag) }
func GetPortManagerImage() string {
	return fmt.Sprintf("%s/%s:%s", repo, portManagerImage, portManagerTag)
}
func GetProxyImage() string { return fmt.Sprintf("%s/%s:%s", repo, proxyImage, proxyTag) }
