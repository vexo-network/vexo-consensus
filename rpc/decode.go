package rpc

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/vexo-network/vexo-consensus/node"
	"github.com/vexo-network/vexo-consensus/slashing"
	"github.com/vexo-network/vexo-consensus/types"
)

func decodeSubmitTxRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (types.Tx, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	var payload SubmitTxRequest
	if err := decodeStrictJSON(request.Body, &payload); err != nil {
		return nil, fmt.Errorf("invalid transaction request: %w", err)
	}
	if payload.Tx == "" {
		return nil, errors.New("transaction is required")
	}
	encoding := payload.Encoding
	if encoding == "" {
		encoding = "base64"
	}
	var tx []byte
	switch encoding {
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(payload.Tx)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 transaction: %w", err)
		}
		tx = decoded
	case "plain":
		tx = []byte(payload.Tx)
	default:
		return nil, fmt.Errorf("unsupported transaction encoding %q", payload.Encoding)
	}
	if len(tx) == 0 {
		return nil, errors.New("transaction is empty")
	}
	return types.Tx(tx), nil
}

func decodeSubmitEvidenceRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (slashing.Evidence, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	var payload SubmitEvidenceRequest
	if err := decodeStrictJSON(request.Body, &payload); err != nil {
		return slashing.Evidence{}, fmt.Errorf("invalid evidence request: %w", err)
	}
	if payload.Type == "" {
		return slashing.Evidence{}, errors.New("evidence type is required")
	}
	if payload.Validator == "" {
		return slashing.Evidence{}, errors.New("evidence validator is required")
	}
	if payload.Height == 0 {
		return slashing.Evidence{}, errors.New("evidence height is required")
	}
	if payload.Proof == "" {
		return slashing.Evidence{}, errors.New("evidence proof is required")
	}
	encoding := payload.Encoding
	if encoding == "" {
		encoding = "base64"
	}
	var proof []byte
	switch encoding {
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(payload.Proof)
		if err != nil {
			return slashing.Evidence{}, fmt.Errorf("invalid base64 evidence proof: %w", err)
		}
		proof = decoded
	case "plain":
		proof = []byte(payload.Proof)
	default:
		return slashing.Evidence{}, fmt.Errorf("unsupported evidence proof encoding %q", payload.Encoding)
	}
	if len(proof) == 0 {
		return slashing.Evidence{}, errors.New("evidence proof is empty")
	}
	return slashing.Evidence{
		Type:      slashing.EvidenceType(payload.Type),
		Validator: types.ValidatorID(payload.Validator),
		Height:    types.Height(payload.Height),
		Round:     types.Round(payload.Round),
		Proof:     proof,
	}, nil
}

func decodePruneRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (types.Height, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	var payload PruneRequest
	if err := decodeStrictJSON(request.Body, &payload); err != nil {
		return 0, fmt.Errorf("invalid prune request: %w", err)
	}
	if payload.RetainFromHeight == 0 {
		return 0, errors.New("retain_from_height is required")
	}
	return types.Height(payload.RetainFromHeight), nil
}

func decodeReplayRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (ReplayRequest, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	var payload ReplayRequest
	if err := decodeStrictJSON(request.Body, &payload); err != nil {
		return ReplayRequest{}, fmt.Errorf("invalid replay request: %w", err)
	}
	if payload.All && (payload.FromHeight != 0 || payload.ToHeight != 0) {
		return ReplayRequest{}, errors.New("all cannot be combined with from_height or to_height")
	}
	return payload, nil
}

func decodeConsensusLoopRequest(writer http.ResponseWriter, request *http.Request, maxRequestBytes int64) (node.ConsensusLoopConfig, error) {
	request.Body = http.MaxBytesReader(writer, request.Body, maxRequestBytes)
	defer request.Body.Close()

	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var payload ConsensusLoopRequest
	if err := decoder.Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			return node.ConsensusLoopConfig{}, nil
		}
		return node.ConsensusLoopConfig{}, fmt.Errorf("invalid consensus loop request: %w", err)
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return node.ConsensusLoopConfig{}, errors.New("invalid consensus loop request: trailing JSON data")
	}
	return node.ConsensusLoopConfig{
		Interval:      time.Duration(payload.IntervalMillis) * time.Millisecond,
		RoundTimeout:  time.Duration(payload.RoundTimeoutMillis) * time.Millisecond,
		MaxBlockBytes: payload.MaxBlockBytes,
	}, nil
}

func decodeStrictJSON(reader io.Reader, value any) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("trailing JSON data")
	}
	return nil
}
