package u2fhost

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	butil "github.com/marshallbrekka/go-u2fhost/bytes"
)

// Authenticates with the device using the AuthenticateRequest,
// returning an AuthenticateResponse.
func (dev *HidDevice) Authenticate(req *AuthenticateRequest) (*AuthenticateResponse, error) {
	clientData, request, err := authenticateRequest(req)
	if err != nil {
		return nil, err
	}

	authModifier := u2fAuthEnforce
	if req.CheckOnly {
		authModifier = u2fAuthCheckOnly
	}
	status, response, err := dev.hidDevice.SendAPDU(
		u2fCommandAuthenticate, authModifier, 0, request)
	return authenticateResponse(status, response, clientData, req.KeyHandle, req.AppId, req.WebAuthn, err)
}

func authenticateResponse(status uint16, response, clientData []byte, keyHandle string, appID string, webAuthn bool, err error) (*AuthenticateResponse, error) {
	var authenticateResponse *AuthenticateResponse
	if err == nil {
		if status == u2fStatusNoError {
			if webAuthn {
				authenticatorData := append(sha256([]byte(appID)), response[0:5]...)
				authenticateResponse = &AuthenticateResponse{
					KeyHandle:         keyHandle,
					ClientData:        websafeEncode(clientData),
					SignatureData:     base64.StdEncoding.EncodeToString(response[5:]),
					AuthenticatorData: base64.StdEncoding.EncodeToString(authenticatorData),
				}
			} else {
				authenticateResponse = &AuthenticateResponse{
					KeyHandle:     keyHandle,
					ClientData:    websafeEncode(clientData),
					SignatureData: websafeEncode(response),
				}
			}

		} else {
			err = u2ferror(status)
		}
	}
	return authenticateResponse, err
}

func authenticateRequest(req *AuthenticateRequest) ([]byte, []byte, error) {
	// Get the channel id public key, if any
	cid, err := channelIdPublicKey(req.ChannelIdPublicKey, req.ChannelIdUnused)
	if err != nil {
		return nil, nil, err
	}

	// Construct the client json
	keyHandle, err := websafeDecode(req.KeyHandle)
	if err != nil {
		return []byte{}, []byte{}, fmt.Errorf("base64 key handle: %s", err)
	}
	typ := "navigator.id.getAssertion"
	if req.WebAuthn {
		typ = "webauthn.get"
	}
	client := clientData{
		Type:               typ,
		Challenge:          req.Challenge,
		Origin:             req.Facet,
		ChannelIdPublicKey: cid,
	}

	clientJson, err := json.Marshal(client)
	if err != nil {
		return nil, nil, fmt.Errorf("Error marshaling clientData to json: %s", err)
	}

	// Pack into byte array
	// https://fidoalliance.org/specs/fido-u2f-v1.0-nfc-bt-amendment-20150514/fido-u2f-raw-message-formats.html#authentication-request-message---u2f_authenticate
	request := butil.Concat(
		sha256(clientJson),
		sha256([]byte(req.AppId)),
		[]byte{byte(len(keyHandle))},
		keyHandle,
	)
	return []byte(clientJson), request, nil
}
