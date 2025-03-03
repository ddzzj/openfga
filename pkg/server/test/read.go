package test

import (
	"context"
	"testing"

	parser "github.com/craigpastro/openfga-dsl-parser/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/oklog/ulid/v2"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"github.com/openfga/openfga/pkg/encoder"
	"github.com/openfga/openfga/pkg/encrypter"
	"github.com/openfga/openfga/pkg/logger"
	"github.com/openfga/openfga/pkg/server/commands"
	serverErrors "github.com/openfga/openfga/pkg/server/errors"
	"github.com/openfga/openfga/pkg/storage"
	"github.com/openfga/openfga/pkg/testutils"
	"github.com/openfga/openfga/pkg/tuple"
	"github.com/openfga/openfga/pkg/typesystem"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func ReadQuerySuccessTest(t *testing.T, datastore storage.OpenFGADatastore) {
	// TODO: review which of these tests should be moved to validation/types in grpc rather than execution. e.g.: invalid relation in authorizationmodel is fine, but tuple without authorizationmodel is should be required before. see issue: https://github.com/openfga/sandcastle/issues/13
	tests := []struct {
		_name    string
		model    *openfgav1.AuthorizationModel
		tuples   []*openfgav1.TupleKey
		request  *openfgav1.ReadRequest
		response *openfgav1.ReadResponse
	}{
		{
			_name: "ExecuteReturnsExactMatchingTupleKey",
			// state
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: parser.MustParse(`
				type user

				type team

				type repo
				  relations
				    define owner: [team] as self
				    define admin: [user] as self
				`),
			},
			tuples: []*openfgav1.TupleKey{
				{
					Object:   "repo:openfga/openfga",
					Relation: "admin",
					User:     "user:github|jose",
				},
				{
					Object:   "repo:openfga/openfga",
					Relation: "owner",
					User:     "team:iam",
				},
			},
			// input
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object:   "repo:openfga/openfga",
					Relation: "admin",
					User:     "user:github|jose",
				},
			},
			// output
			response: &openfgav1.ReadResponse{
				Tuples: []*openfgav1.Tuple{
					{
						Key: &openfgav1.TupleKey{
							Object:   "repo:openfga/openfga",
							Relation: "admin",
							User:     "user:github|jose",
						},
					},
				},
			},
		},
		{
			_name: "ExecuteReturnsTuplesWithProvidedUserAndObjectIdInAuthorizationModelRegardlessOfRelationIfNoRelation",
			// state
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_1,
				TypeDefinitions: parser.MustParse(`
				type user

				type repo
				  relations
				    define admin: [user] as self
				    define owner: [user] as self
				`),
			},
			tuples: []*openfgav1.TupleKey{
				tuple.NewTupleKey("repo:openfga/openfga", "admin", "user:github|jose"),
				tuple.NewTupleKey("repo:openfga/openfga", "owner", "user:github|jose"),
			},
			// input
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object: "repo:openfga/openfga",
					User:   "user:github|jose",
				},
			},
			// output
			response: &openfgav1.ReadResponse{
				Tuples: []*openfgav1.Tuple{
					{Key: tuple.NewTupleKey("repo:openfga/openfga", "admin", "user:github|jose")},
					{Key: tuple.NewTupleKey("repo:openfga/openfga", "owner", "user:github|jose")},
				},
			},
		},
		{
			_name: "ExecuteReturnsTuplesWithProvidedUserInAuthorizationModelRegardlessOfRelationAndObjectIdIfNoRelationAndNoObjectId",
			// state
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin":  {},
							"writer": {},
						},
					},
				},
			},
			tuples: []*openfgav1.TupleKey{
				{
					Object:   "repo:openfga/openfga",
					Relation: "admin",
					User:     "github|jose",
				},
				{
					Object:   "repo:openfga/openfga-server",
					Relation: "writer",
					User:     "github|jose",
				},
				{
					Object:   "org:openfga",
					Relation: "member",
					User:     "github|jose",
				},
			},
			// input
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object: "repo:",
					User:   "github|jose",
				},
			},
			// output
			response: &openfgav1.ReadResponse{
				Tuples: []*openfgav1.Tuple{
					{Key: &openfgav1.TupleKey{
						Object:   "repo:openfga/openfga",
						Relation: "admin",
						User:     "github|jose",
					}},
					{Key: &openfgav1.TupleKey{
						Object:   "repo:openfga/openfga-server",
						Relation: "writer",
						User:     "github|jose",
					}},
				},
			},
		},
		{
			_name: "ExecuteReturnsTuplesWithProvidedUserAndRelationInAuthorizationModelRegardlessOfObjectIdIfNoObjectId",
			// state
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin":  {},
							"writer": {},
						},
					},
				},
			},
			tuples: []*openfgav1.TupleKey{
				{
					Object:   "repo:openfga/openfga",
					Relation: "admin",
					User:     "github|jose",
				},
				{
					Object:   "repo:openfga/openfga-server",
					Relation: "writer",
					User:     "github|jose",
				},
				{
					Object:   "repo:openfga/openfga-users",
					Relation: "writer",
					User:     "github|jose",
				},
				{
					Object:   "org:openfga",
					Relation: "member",
					User:     "github|jose",
				},
			},
			// input
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object:   "repo:",
					Relation: "writer",
					User:     "github|jose",
				},
			},
			// output
			response: &openfgav1.ReadResponse{
				Tuples: []*openfgav1.Tuple{
					{Key: &openfgav1.TupleKey{
						Object:   "repo:openfga/openfga-server",
						Relation: "writer",
						User:     "github|jose",
					}},
					{Key: &openfgav1.TupleKey{
						Object:   "repo:openfga/openfga-users",
						Relation: "writer",
						User:     "github|jose",
					}},
				},
			},
		},
		{
			_name: "ExecuteReturnsTuplesWithProvidedObjectIdAndRelationInAuthorizationModelRegardlessOfUser",
			// state
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin":  {},
							"writer": {},
						},
					},
				},
			},
			tuples: []*openfgav1.TupleKey{
				{
					Object:   "repo:openfga/openfga",
					Relation: "admin",
					User:     "github|jose",
				},
				{
					Object:   "repo:openfga/openfga",
					Relation: "admin",
					User:     "github|yenkel",
				},
				{
					Object:   "repo:openfga/openfga-users",
					Relation: "writer",
					User:     "github|jose",
				},
				{
					Object:   "org:openfga",
					Relation: "member",
					User:     "github|jose",
				},
			},
			// input
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object:   "repo:openfga/openfga",
					Relation: "admin",
				},
			},
			// output
			response: &openfgav1.ReadResponse{
				Tuples: []*openfgav1.Tuple{
					{Key: &openfgav1.TupleKey{
						Object:   "repo:openfga/openfga",
						Relation: "admin",
						User:     "github|jose",
					}},
					{Key: &openfgav1.TupleKey{
						Object:   "repo:openfga/openfga",
						Relation: "admin",
						User:     "github|yenkel",
					}},
				},
			},
		},
		{
			_name: "ExecuteReturnsTuplesWithProvidedObjectIdInAuthorizationModelRegardlessOfUserAndRelation",
			// state
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin":  {},
							"writer": {},
						},
					},
				},
			},
			tuples: []*openfgav1.TupleKey{
				{
					Object:   "repo:openfga/openfga",
					Relation: "admin",
					User:     "github|jose",
				},
				{
					Object:   "repo:openfga/openfga",
					Relation: "writer",
					User:     "github|yenkel",
				},
				{
					Object:   "repo:openfga/openfga-users",
					Relation: "writer",
					User:     "github|jose",
				},
				{
					Object:   "org:openfga",
					Relation: "member",
					User:     "github|jose",
				},
			},
			// input
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object: "repo:openfga/openfga",
				},
			},
			// output
			response: &openfgav1.ReadResponse{
				Tuples: []*openfgav1.Tuple{
					{Key: &openfgav1.TupleKey{
						Object:   "repo:openfga/openfga",
						Relation: "admin",
						User:     "github|jose",
					}},
					{Key: &openfgav1.TupleKey{
						Object:   "repo:openfga/openfga",
						Relation: "writer",
						User:     "github|yenkel",
					}},
				},
			},
		},
	}

	require := require.New(t)
	ctx := context.Background()
	logger := logger.NewNoopLogger()
	encoder := encoder.NewBase64Encoder()

	for _, test := range tests {
		t.Run(test._name, func(t *testing.T) {
			store := ulid.Make().String()
			err := datastore.WriteAuthorizationModel(ctx, store, test.model)
			require.NoError(err)

			if test.tuples != nil {
				err = datastore.Write(ctx, store, []*openfgav1.TupleKey{}, test.tuples)
				require.NoError(err)
			}

			test.request.StoreId = store
			resp, err := commands.NewReadQuery(datastore, logger, encoder).Execute(ctx, test.request)
			require.NoError(err)

			if test.response.Tuples != nil {
				require.Equal(len(test.response.Tuples), len(resp.Tuples))

				for i, responseTuple := range test.response.Tuples {
					responseTupleKey := responseTuple.Key
					actualTupleKey := resp.Tuples[i].Key
					require.Equal(responseTupleKey.Object, actualTupleKey.Object)
					require.Equal(responseTupleKey.Relation, actualTupleKey.Relation)
					require.Equal(responseTupleKey.User, actualTupleKey.User)
				}
			}
		})
	}
}

func ReadQueryErrorTest(t *testing.T, datastore storage.OpenFGADatastore) {
	// TODO: review which of these tests should be moved to validation/types in grpc rather than execution. e.g.: invalid relation in authorizationmodel is fine, but tuple without authorizationmodel is should be required before. see issue: https://github.com/openfga/sandcastle/issues/13
	tests := []struct {
		_name   string
		model   *openfgav1.AuthorizationModel
		request *openfgav1.ReadRequest
	}{
		{
			_name: "ExecuteErrorsIfOneTupleKeyHasObjectWithoutType",
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin": {},
						},
					},
				},
			},
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object: "openfga/iam",
				},
			},
		},
		{
			_name: "ExecuteErrorsIfOneTupleKeyObjectIs':'",
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin": {},
						},
					},
				},
			},
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object: ":",
				},
			},
		},
		{
			_name: "ErrorIfRequestHasNoObjectAndThusNoType",
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin": {},
						},
					},
				},
			},
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Relation: "admin",
					User:     "github|jonallie",
				},
			},
		},
		{
			_name: "ExecuteErrorsIfOneTupleKeyHasNoObjectIdAndNoUserSetButHasAType",
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin": {},
						},
					},
				},
			},
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object:   "repo:",
					Relation: "writer",
				},
			},
		},
		{
			_name: "ExecuteErrorsIfOneTupleKeyInTupleSetOnlyHasRelation",
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin": {},
						},
					},
				},
			},
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Relation: "writer",
				},
			},
		},
		{
			_name: "ExecuteErrorsIfContinuationTokenIsBad",
			model: &openfgav1.AuthorizationModel{
				Id:            ulid.Make().String(),
				SchemaVersion: typesystem.SchemaVersion1_0,
				TypeDefinitions: []*openfgav1.TypeDefinition{
					{
						Type: "repo",
						Relations: map[string]*openfgav1.Userset{
							"admin":  {},
							"writer": {},
						},
					},
				},
			},
			request: &openfgav1.ReadRequest{
				TupleKey: &openfgav1.TupleKey{
					Object: "repo:openfga/openfga",
				},
				ContinuationToken: "foo",
			},
		},
	}

	require := require.New(t)
	ctx := context.Background()
	logger := logger.NewNoopLogger()
	encoder := encoder.NewBase64Encoder()

	for _, test := range tests {
		t.Run(test._name, func(t *testing.T) {
			store := ulid.Make().String()
			err := datastore.WriteAuthorizationModel(ctx, store, test.model)
			require.NoError(err)

			test.request.StoreId = store
			_, err = commands.NewReadQuery(datastore, logger, encoder).Execute(ctx, test.request)
			require.Error(err)
		})
	}
}

func ReadAllTuplesTest(t *testing.T, datastore storage.OpenFGADatastore) {
	ctx := context.Background()
	logger := logger.NewNoopLogger()
	store := ulid.Make().String()

	writes := []*openfgav1.TupleKey{
		{
			Object:   "repo:openfga/foo",
			Relation: "admin",
			User:     "github|jon.allie",
		},
		{
			Object:   "repo:openfga/bar",
			Relation: "admin",
			User:     "github|jon.allie",
		},
		{
			Object:   "repo:openfga/baz",
			Relation: "admin",
			User:     "github|jon.allie",
		},
	}
	err := datastore.Write(ctx, store, nil, writes)
	require.NoError(t, err)

	cmd := commands.NewReadQuery(datastore, logger, encoder.NewBase64Encoder())

	firstRequest := &openfgav1.ReadRequest{
		StoreId:           store,
		PageSize:          wrapperspb.Int32(1),
		ContinuationToken: "",
	}
	firstResponse, err := cmd.Execute(ctx, firstRequest)
	require.NoError(t, err)

	require.Len(t, firstResponse.Tuples, 1)
	require.NotEmpty(t, firstResponse.ContinuationToken)

	var receivedTuples []*openfgav1.TupleKey
	for _, tuple := range firstResponse.Tuples {
		receivedTuples = append(receivedTuples, tuple.Key)
	}

	secondRequest := &openfgav1.ReadRequest{StoreId: store, ContinuationToken: firstResponse.ContinuationToken}
	secondResponse, err := cmd.Execute(ctx, secondRequest)
	require.NoError(t, err)

	require.Len(t, secondResponse.Tuples, 2)
	require.Empty(t, secondResponse.ContinuationToken)

	for _, tuple := range secondResponse.Tuples {
		receivedTuples = append(receivedTuples, tuple.Key)
	}

	cmpOpts := []cmp.Option{
		cmpopts.IgnoreUnexported(openfgav1.TupleKey{}, openfgav1.Tuple{}, openfgav1.TupleChange{}, openfgav1.Assertion{}),
		cmpopts.IgnoreFields(openfgav1.Tuple{}, "Timestamp"),
		cmpopts.IgnoreFields(openfgav1.TupleChange{}, "Timestamp"),
		testutils.TupleKeyCmpTransformer,
	}

	if diff := cmp.Diff(writes, receivedTuples, cmpOpts...); diff != "" {
		t.Errorf("Tuple mismatch (-want +got):\n%s", diff)
	}
}

func ReadAllTuplesInvalidContinuationTokenTest(t *testing.T, datastore storage.OpenFGADatastore) {
	ctx := context.Background()
	logger := logger.NewNoopLogger()
	store := ulid.Make().String()

	encrypter, err := encrypter.NewGCMEncrypter("key")
	require.NoError(t, err)

	encoder := encoder.NewTokenEncoder(encrypter, encoder.NewBase64Encoder())

	model := &openfgav1.AuthorizationModel{
		Id:            ulid.Make().String(),
		SchemaVersion: typesystem.SchemaVersion1_0,
		TypeDefinitions: []*openfgav1.TypeDefinition{
			{
				Type: "repo",
			},
		},
	}

	err = datastore.WriteAuthorizationModel(ctx, store, model)
	require.NoError(t, err)

	_, err = commands.NewReadQuery(datastore, logger, encoder).Execute(ctx, &openfgav1.ReadRequest{
		StoreId:           store,
		ContinuationToken: "foo",
	})
	require.ErrorIs(t, err, serverErrors.InvalidContinuationToken)
}
