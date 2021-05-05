// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package directinvoke

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestCustomerHeadersEmpty(t *testing.T) {
	in := CustomerHeaders{}
	out := CustomerHeaders{}

	require.NoError(t, out.Load(in.Dump()))
	require.Equal(t, in, out)
}

func TestCustomerHeaders(t *testing.T) {
	in := CustomerHeaders{CognitoIdentityID: "asd"}
	out := CustomerHeaders{}

	require.NoError(t, out.Load(in.Dump()))
	require.Equal(t, in, out)
}
