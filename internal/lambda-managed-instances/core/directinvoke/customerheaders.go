// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package directinvoke

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
)

type CustomerHeaders struct {
	CognitoIdentityID     string `json:"Cognito-Identity-Id"`
	CognitoIdentityPoolID string `json:"Cognito-Identity-Pool-Id"`
	ClientContext         string `json:"Client-Context"`
}

func (s CustomerHeaders) Dump() string {
	if (s == CustomerHeaders{}) {
		return ""
	}

	custHeadersJSON, err := json.Marshal(&s)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(custHeadersJSON)
}

func (s *CustomerHeaders) Load(in string) error {
	*s = CustomerHeaders{}

	if in == "" {
		return nil
	}

	base64Decoder := base64.NewDecoder(base64.StdEncoding, bytes.NewReader([]byte(in)))

	return json.NewDecoder(base64Decoder).Decode(s)
}
