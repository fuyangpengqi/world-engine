package iterator_test

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"

	"pkg.world.dev/world-engine/assert"
	"pkg.world.dev/world-engine/cardinal"
	"pkg.world.dev/world-engine/cardinal/router/iterator"
	"pkg.world.dev/world-engine/cardinal/types"
	shard "pkg.world.dev/world-engine/rift/shard/v2"
)

var _ shard.TransactionHandlerClient = &mockQuerier{}
var fooMsg = cardinal.NewMessageType[fooIn, fooOut]("foo")

type fooIn struct{ X int }
type fooOut struct{}

type mockQuerier struct {
	i       int
	retErr  error
	ret     []*shard.QueryTransactionsResponse
	request *shard.QueryTransactionsRequest
}

func (m *mockQuerier) RegisterGameShard(
	_ context.Context, _ *shard.RegisterGameShardRequest, _ ...grpc.CallOption) (
	*shard.RegisterGameShardResponse, error) {
	panic("intentionally not implemented. this is a mock.")
}

func (m *mockQuerier) Submit(_ context.Context, _ *shard.SubmitTransactionsRequest, _ ...grpc.CallOption) (
	*shard.SubmitTransactionsResponse, error) {
	panic("intentionally not implemented. this is a mock.")
}

// this mock will return its error, if set, otherwise, it will return whatever is in ret[i], where i represents the
// amount of times this was called.
func (m *mockQuerier) QueryTransactions(
	_ context.Context,
	req *shard.QueryTransactionsRequest,
	_ ...grpc.CallOption,
) (*shard.QueryTransactionsResponse, error) {
	m.request = req
	if m.retErr != nil {
		return nil, m.retErr
	}
	defer func() { m.i++ }()
	return m.ret[m.i], nil
}

func TestIteratorReturnsErrorWhenQueryNotFound(t *testing.T) {
	querier := &mockQuerier{
		ret: []*shard.QueryTransactionsResponse{
			{
				Epochs: []*shard.Epoch{
					{
						Epoch:         1,
						UnixTimestamp: 1,
						Txs: []*shard.TxData{
							{
								TxId:                 1,
								GameShardTransaction: nil,
							},
						},
					},
				},
			},
		},
	}
	it := iterator.New(
		func(types.MessageID) (types.Message, bool) {
			return nil, false
		},
		"",
		querier,
	)
	err := it.Each(nil)
	assert.ErrorContains(t, err, "queried message with ID 1, but it does not exist in Cardinal")
}

func TestIteratorReturnsErrorIfQueryFails(t *testing.T) {
	querier := &mockQuerier{retErr: errors.New("some error")}
	it := iterator.New(nil, "foo", querier)
	err := it.Each(nil)
	assert.ErrorContains(t, err, "some error")
}

func TestIteratorHappyPath(t *testing.T) {
	err := fooMsg.SetID(10)
	assert.NilError(t, err)
	namespace := "ns"
	msgValue := fooIn{3}
	msgBytes, err := fooMsg.Encode(msgValue)
	assert.NilError(t, err)
	protoTx := &shard.Transaction{
		PersonaTag: "ty",
		Namespace:  namespace,
		Timestamp:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		Signature:  "fo",
		Body:       msgBytes,
	}
	txBz, err := proto.Marshal(protoTx)
	assert.NilError(t, err)
	querier := &mockQuerier{
		ret: []*shard.QueryTransactionsResponse{
			{
				Epochs: []*shard.Epoch{
					{
						Epoch:         12,
						UnixTimestamp: 15,
						Txs: []*shard.TxData{
							{
								TxId:                 uint64(fooMsg.ID()),
								GameShardTransaction: txBz,
							},
						},
					},
				},
				Page: &shard.PageResponse{},
			},
		},
	}
	it := iterator.New(
		func(id types.MessageID) (types.Message, bool) {
			if id == fooMsg.ID() {
				return fooMsg, true
			}
			return nil, false
		},
		namespace,
		querier,
	)
	err = it.Each(func(batch []*iterator.TxBatch, tick, timestamp uint64) error {
		assert.Len(t, batch, 1)
		assert.Equal(t, tick, uint64(12))
		assert.Equal(t, timestamp, uint64(15))
		tx := batch[0]

		assert.Equal(t, tx.MsgValue, msgValue)
		assert.Equal(t, tx.MsgID, fooMsg.ID())
		assert.Equal(t, tx.Tx.PersonaTag, protoTx.GetPersonaTag())
		assert.Equal(t, tx.Tx.Timestamp, time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
		assert.True(t, len(tx.Tx.Hash.Bytes()) > 1)
		assert.Equal(t, tx.Tx.Namespace, namespace)
		assert.DeepEqual(t, []byte(tx.Tx.Body), msgBytes)

		return nil
	})
	assert.NilError(t, err)
}

func TestIteratorStartRange(t *testing.T) {
	querier := &mockQuerier{retErr: errors.New("whatever")}
	it := iterator.New(nil, "", querier)

	// we don't care about this error, we're just checking if `querier` gets called with the right key in the Page.
	startRange := uint64(5)
	_ = it.Each(nil, 5)

	req := querier.request
	gotStartRange := parsePageKey(req.GetPage().GetKey())
	assert.Equal(t, startRange, gotStartRange)
}

func TestIteratorStopRange(t *testing.T) {
	err := fooMsg.SetID(10)
	assert.NilError(t, err)
	namespace := "ns"
	msgValue := fooIn{3}
	msgBytes, err := fooMsg.Encode(msgValue)
	assert.NilError(t, err)
	protoTx := &shard.Transaction{
		PersonaTag: "ty",
		Namespace:  namespace,
		Timestamp:  time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		Signature:  "fo",
		Body:       msgBytes,
	}
	txBz, err := proto.Marshal(protoTx)
	assert.NilError(t, err)
	querier := &mockQuerier{
		ret: []*shard.QueryTransactionsResponse{
			{
				Epochs: []*shard.Epoch{
					{
						Epoch:         12,
						UnixTimestamp: 15,
						Txs: []*shard.TxData{
							{
								TxId:                 uint64(fooMsg.ID()),
								GameShardTransaction: txBz,
							},
						},
					},
					{
						Epoch: 20,
					},
				},
				Page: &shard.PageResponse{},
			},
		},
	}
	it := iterator.New(
		func(id types.MessageID) (types.Message, bool) {
			if id == fooMsg.ID() {
				return fooMsg, true
			}
			return nil, false
		},
		namespace,
		querier,
	)
	called := 0
	err = it.Each(func(_ []*iterator.TxBatch, _, _ uint64) error {
		called++
		return nil
	}, 0, 15)
	assert.NilError(t, err)
	assert.Equal(t, called, 1)
}

func TestStartGreaterThanStopRange(t *testing.T) {
	it := iterator.New(nil, "", nil)
	err := it.Each(nil, 154, 0)
	assert.ErrorContains(t, err, "first number in range must be less than the second (start,stop)")
}

func parsePageKey(key []byte) uint64 {
	tick := binary.BigEndian.Uint64(key)
	return tick
}
