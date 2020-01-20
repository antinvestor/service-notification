package utils

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type (
	//JWTInterceptor struct
	JWTInterceptor struct {
		http     *http.Client // The HTTP client for calling the token-serving API
		token    string       // The JWT token that will be used in every call to the server
		username string       // The username for basic authentication
		password string       // The password for basic authentication
		endpoint string       // The HTTP endpoint to hit to obtain tokens
	}

	authResponse struct {
		Token string `json:"token"`
	}

	authRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
)

func (jwt *JWTInterceptor) refreshBearerToken() error {
	resp, err := jwt.performAuthRequest()

	if err != nil {
		return err
	}

	var respBody authResponse
	if err = json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return err
	}

	jwt.token = respBody.Token

	return resp.Body.Close()
}

func (jwt *JWTInterceptor) performAuthRequest() (*http.Response, error) {
	body := authRequest{
		Username: jwt.username,
		Password: jwt.password,
	}

	data, err := json.Marshal(body)

	if err != nil {
		return nil, err
	}

	buff := bytes.NewBuffer(data)
	resp, err := jwt.http.Post(jwt.endpoint, "application/json", buff)

	if err != nil {
		return resp, err
	}

	if resp.StatusCode != http.StatusOK {
		out := make([]byte, resp.ContentLength)
		if _, err = resp.Body.Read(out); err != nil {
			return resp, err
		}

		return resp, fmt.Errorf("unexpected authentication response: %s", string(out))
	}

	return resp, nil
}
//UnaryClientInterceptor mm
func (jwt *JWTInterceptor) UnaryClientInterceptor(ctx context.Context, method string, req interface{}, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	// Create a new context with the token and make the first request
	authCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "bearer "+jwt.token)
	err := invoker(authCtx, method, req, reply, cc, opts...)

	// If we got an unauthenticated response from the gRPC service, refresh the token
	if status.Code(err) == codes.Unauthenticated {
		if err = jwt.refreshBearerToken(); err != nil {
			return err
		}

		// Create a new context with the new token. We don't want to reuse 'authCtx' here
		// because we've already appended the invalid token. We're appending metadata to
		// a slice here rather than a map like HTTP headers, so the first one will be picked
		// up and invalid.
		updatedAuthCtx := metadata.AppendToOutgoingContext(ctx, "authorization", "bearer "+jwt.token)
		err = invoker(updatedAuthCtx, method, req, reply, cc, opts...)
	}

	return err
}


