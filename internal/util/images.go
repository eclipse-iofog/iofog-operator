/*
 *  *******************************************************************************
 *  * Copyright (c) 2023 Datasance Teknoloji A.S.
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

// These values are set by the linker, e.g. "LDFLAGS += -X $(PREFIX).controllerTag=v3.0.0-beta1".
var (
	repo          = "undefined" //nolint:gochecknoglobals
	controllerTag = "undefined" //nolint:gochecknoglobals
	routerTag     = "undefined" //nolint:gochecknoglobals
	natsTag       = "undefined" //nolint:gochecknoglobals
)

const (
	controllerImage = "controller"
	routerImage     = "router"
	natsImage       = "nats"
)

func GetControllerImage() string {
	return fmt.Sprintf("%s/%s:%s", repo, controllerImage, controllerTag)
}
func GetRouterImage() string { return fmt.Sprintf("%s/%s:%s", repo, routerImage, routerTag) }
func GetNatsImage() string   { return fmt.Sprintf("%s/%s:%s", repo, natsImage, natsTag) }
