package test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/openfga/pkg/encoder"
	"github.com/openfga/openfga/pkg/encrypter"
	"github.com/openfga/openfga/pkg/logger"
	"github.com/openfga/openfga/pkg/server/commands"
	serverErrors "github.com/openfga/openfga/pkg/server/errors"
	"github.com/openfga/openfga/pkg/storage"
	"github.com/openfga/openfga/pkg/testutils"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type testCase struct {
	_name                            string
	request                          *openfgav1.ReadChangesRequest
	expectedError                    error
	expectedChanges                  []*openfgav1.TupleChange
	expectEmptyContinuationToken     bool
	saveContinuationTokenForNextTest bool
}

var tkMaria = &openfgav1.TupleKey{
	Object:   "repo:openfga/openfgapb",
	Relation: "admin",
	User:     "maria",
}
var tkMariaOrg = &openfgav1.TupleKey{
	Object:   "org:openfga",
	Relation: "member",
	User:     "maria",
}
var tkCraig = &openfgav1.TupleKey{
	Object:   "repo:openfga/openfgapb",
	Relation: "admin",
	User:     "craig",
}
var tkYamil = &openfgav1.TupleKey{
	Object:   "repo:openfga/openfgapb",
	Relation: "admin",
	User:     "yamil",
}

func newReadChangesRequest(store, objectType, contToken string, pageSize int32) *openfgav1.ReadChangesRequest {
	return &openfgav1.ReadChangesRequest{
		StoreId:           store,
		Type:              objectType,
		ContinuationToken: contToken,
		PageSize:          wrapperspb.Int32(pageSize),
	}
}

func TestReadChanges(t *testing.T, datastore storage.OpenFGADatastore) {
	store := testutils.CreateRandomString(10)
	ctx, backend, err := setup(store, datastore)
	require.NoError(t, err)

	encrypter, err := encrypter.NewGCMEncrypter("key")
	require.NoError(t, err)

	encoder := encoder.NewTokenEncoder(encrypter, encoder.NewBase64Encoder())

	t.Run("read_changes_without_type", func(t *testing.T) {
		testCases := []testCase{
			{
				_name:   "request_with_pageSize=2_returns_2_tuple_and_a_token",
				request: newReadChangesRequest(store, "", "", 2),
				expectedChanges: []*openfgav1.TupleChange{
					{
						TupleKey:  tkMaria,
						Operation: openfgav1.TupleOperation_TUPLE_OPERATION_WRITE,
					},
					{
						TupleKey:  tkCraig,
						Operation: openfgav1.TupleOperation_TUPLE_OPERATION_WRITE,
					},
				},
				expectEmptyContinuationToken:     false,
				expectedError:                    nil,
				saveContinuationTokenForNextTest: true,
			},
			{
				_name:   "request_with_previous_token_returns_all_remaining_changes",
				request: newReadChangesRequest(store, "", "", storage.DefaultPageSize),
				expectedChanges: []*openfgav1.TupleChange{
					{
						TupleKey:  tkYamil,
						Operation: openfgav1.TupleOperation_TUPLE_OPERATION_WRITE,
					},
					{
						TupleKey:  tkMariaOrg,
						Operation: openfgav1.TupleOperation_TUPLE_OPERATION_WRITE,
					},
				},
				expectEmptyContinuationToken:     false,
				expectedError:                    nil,
				saveContinuationTokenForNextTest: true,
			},
			{
				_name:                            "request_with_previous_token_returns_no_more_changes",
				request:                          newReadChangesRequest(store, "", "", storage.DefaultPageSize),
				expectedChanges:                  nil,
				expectEmptyContinuationToken:     false,
				expectedError:                    nil,
				saveContinuationTokenForNextTest: false,
			},
			{
				_name:                            "request_with_invalid_token_returns_invalid_token_error",
				request:                          newReadChangesRequest(store, "", "foo", storage.DefaultPageSize),
				expectedChanges:                  nil,
				expectEmptyContinuationToken:     false,
				expectedError:                    serverErrors.InvalidContinuationToken,
				saveContinuationTokenForNextTest: false,
			},
		}

		readChangesQuery := commands.NewReadChangesQuery(backend, logger.NewNoopLogger(), encoder, 0)
		runTests(t, ctx, testCases, readChangesQuery)
	})

	t.Run("read_changes_with_type", func(t *testing.T) {
		testCases := []testCase{
			{
				_name:                        "if_no_tuples_with_type,_return_empty_changes_and_no_token",
				request:                      newReadChangesRequest(store, "type-not-found", "", 1),
				expectedChanges:              nil,
				expectEmptyContinuationToken: true,
				expectedError:                nil,
			},
			{
				_name:   "if_1_tuple_with_'org type',_read_changes_with_'org'_filter_returns_1_change_and_a_token",
				request: newReadChangesRequest(store, "org", "", storage.DefaultPageSize),
				expectedChanges: []*openfgav1.TupleChange{{
					TupleKey:  tkMariaOrg,
					Operation: openfgav1.TupleOperation_TUPLE_OPERATION_WRITE,
				}},
				expectEmptyContinuationToken: false,
				expectedError:                nil,
			},
			{
				_name:   "if_2_tuples_with_'repo'_type,_read_changes_with_'repo'_filter and page size of 1 returns 1 change and a token",
				request: newReadChangesRequest(store, "repo", "", 1),
				expectedChanges: []*openfgav1.TupleChange{{
					TupleKey:  tkMaria,
					Operation: openfgav1.TupleOperation_TUPLE_OPERATION_WRITE,
				}},
				expectEmptyContinuationToken:     false,
				expectedError:                    nil,
				saveContinuationTokenForNextTest: true,
			}, {
				_name:   "using_the_token_from_the_previous_test_yields_1_change_and_a_token",
				request: newReadChangesRequest(store, "repo", "", storage.DefaultPageSize),
				expectedChanges: []*openfgav1.TupleChange{{
					TupleKey:  tkCraig,
					Operation: openfgav1.TupleOperation_TUPLE_OPERATION_WRITE,
				}, {
					TupleKey:  tkYamil,
					Operation: openfgav1.TupleOperation_TUPLE_OPERATION_WRITE,
				}},
				expectEmptyContinuationToken:     false,
				expectedError:                    nil,
				saveContinuationTokenForNextTest: true,
			}, {
				_name:                            "using_the_token_from_the_previous_test_yields_0_changes_and_a_token",
				request:                          newReadChangesRequest(store, "repo", "", storage.DefaultPageSize),
				expectedChanges:                  nil,
				expectEmptyContinuationToken:     false,
				expectedError:                    nil,
				saveContinuationTokenForNextTest: true,
			}, {
				_name:         "using_the_token_from_the_previous_test_yields_an_error_because_the_types_in_the_token_and_the_request_don't_match",
				request:       newReadChangesRequest(store, "does-not-match", "", storage.DefaultPageSize),
				expectedError: serverErrors.MismatchObjectType,
			},
		}

		readChangesQuery := commands.NewReadChangesQuery(backend, logger.NewNoopLogger(), encoder, 0)
		runTests(t, ctx, testCases, readChangesQuery)
	})

	t.Run("read_changes_with_horizon_offset", func(t *testing.T) {
		testCases := []testCase{
			{
				_name: "when_the_horizon_offset_is_non-zero_no_tuples_should_be_returned",
				request: &openfgav1.ReadChangesRequest{
					StoreId: store,
				},
				expectedChanges:              nil,
				expectEmptyContinuationToken: true,
				expectedError:                nil,
			},
		}

		readChangesQuery := commands.NewReadChangesQuery(backend, logger.NewNoopLogger(), encoder, 2)
		runTests(t, ctx, testCases, readChangesQuery)
	})
}

func runTests(t *testing.T, ctx context.Context, testCasesInOrder []testCase, readChangesQuery *commands.ReadChangesQuery) {
	ignoreStateOpts := cmpopts.IgnoreUnexported(openfgav1.Tuple{}, openfgav1.TupleKey{}, openfgav1.TupleChange{})
	ignoreTimestampOpts := cmpopts.IgnoreFields(openfgav1.TupleChange{}, "Timestamp")

	var res *openfgav1.ReadChangesResponse
	var err error
	for i, test := range testCasesInOrder {
		t.Run(test._name, func(t *testing.T) {
			if i >= 1 {
				previousTest := testCasesInOrder[i-1]
				if previousTest.saveContinuationTokenForNextTest {
					previousToken := res.ContinuationToken
					test.request.ContinuationToken = previousToken
				}
			}
			res, err = readChangesQuery.Execute(ctx, test.request)

			if test.expectedError != nil {
				require.ErrorIs(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
				require.NotNil(t, res)
				if diff := cmp.Diff(test.expectedChanges, res.Changes, ignoreStateOpts, ignoreTimestampOpts, cmpopts.EquateEmpty()); diff != "" {
					t.Errorf("tuple change mismatch (-want +got):\n%s", diff)
				}
				if test.expectEmptyContinuationToken {
					require.Empty(t, res.ContinuationToken)
				} else {
					require.NotEmpty(t, res.ContinuationToken)
				}
			}
		})
	}
}

func TestReadChangesReturnsSameContTokenWhenNoChanges(t *testing.T, datastore storage.OpenFGADatastore) {
	store := testutils.CreateRandomString(10)
	ctx, backend, err := setup(store, datastore)
	require.NoError(t, err)

	readChangesQuery := commands.NewReadChangesQuery(backend, logger.NewNoopLogger(), encoder.NewBase64Encoder(), 0)

	res1, err := readChangesQuery.Execute(ctx, newReadChangesRequest(store, "", "", storage.DefaultPageSize))
	require.NoError(t, err)

	res2, err := readChangesQuery.Execute(ctx, newReadChangesRequest(store, "", res1.GetContinuationToken(), storage.DefaultPageSize))
	require.NoError(t, err)

	require.Equal(t, res1.ContinuationToken, res2.ContinuationToken)
}

func setup(store string, datastore storage.OpenFGADatastore) (context.Context, storage.ChangelogBackend, error) {
	ctx := context.Background()

	writes := []*openfgav1.TupleKey{tkMaria, tkCraig, tkYamil, tkMariaOrg}
	err := datastore.Write(ctx, store, []*openfgav1.TupleKey{}, writes)
	if err != nil {
		return nil, nil, err
	}

	return ctx, datastore, nil
}
