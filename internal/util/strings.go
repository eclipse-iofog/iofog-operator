/*
 *  *******************************************************************************
 *  * Copyright (c) 2020 Edgeworx, Inc.
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

import (
	"strings"
)

func Before(input, substr string) string {
	pos := strings.Index(input, substr)
	if pos == -1 {
		return input
	}
	return input[0:pos]
}

func After(input, substr string) string {
	pos := strings.Index(input, substr)
	if pos == -1 || pos+1 > len(input)-1 {
		return ""
	}
	return input[pos+1:]
}

func TransformImageToARM(image string) string {
	if !strings.Contains(image, ":") {
		return image
	}
	return Before(image, ":") + "-arm:" + After(image, ":")
}
