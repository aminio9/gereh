package application

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// TestProvisionTenantWorkflowReplay replays captured production histories to
// guarantee determinism across workflow code changes.
func TestProvisionTenantWorkflowReplay(
	t *testing.T,
) {
	files, err := filepath.Glob(
		"testdata/history/*.json",
	)
	require.NoError(t, err)

	if len(files) == 0 {
		t.Skip("no captured workflow histories")
	}

	replayer := worker.NewWorkflowReplayer()
	replayer.RegisterWorkflowWithOptions(
		ProvisionTenantWorkflow,
		workflow.RegisterOptions{
			Name: ProvisionTenantWorkflowName,
		},
	)

	for _, file := range files {
		file := file

		t.Run(
			filepath.Base(file),
			func(t *testing.T) {
				t.Parallel()

				_, statErr := os.Stat(file)
				require.NoError(t, statErr)

				require.NoError(
					t,
					replayer.ReplayWorkflowHistoryFromJSONFile(
						nil,
						file,
					),
				)
			},
		)
	}
}
