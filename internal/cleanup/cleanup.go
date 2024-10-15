package cleanup

import (
	"context"

	danateamv1 "github.com/matanamar10/namesapcelabel/api/v1"
	"github.com/matanamar10/namesapcelabel/internal/pkg/client"
)

func PerformCleanup(ctx context.Context, client client.NamespaceLabelClient, nsLabel *danateamv1.NamespaceLabel) error {
	return nil
}
