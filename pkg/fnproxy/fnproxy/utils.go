/*
Copyright 2022 Nokia.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fnproxy

/*
import (
	"context"
	"fmt"
	"strings"
	"time"

	fnrunv1alpha1 "github.com/fnrunner/fnruntime/apis/fnrun/v1alpha1"
	"github.com/google/go-containerregistry/pkg/gcrane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func getImageDigestAndEntrypoint(ctx context.Context, image string) (*fnrunv1alpha1.DigestAndEntrypoint, error) {
	l := log.FromContext(ctx)
	start := time.Now()
	defer func() {
		l.Info("getting image metadata", "image", image, "tool", time.Now().Sub(start))
	}()
	var entrypoint []string
	ref, err := name.ParseReference(image)
	if err != nil {
		return nil, err
	}
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(gcrane.Keychain), remote.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	hash, err := img.Digest()
	if err != nil {
		return nil, err
	}
	cf, err := img.ConfigFile()
	if err != nil {
		return nil, err
	}

	cfg := cf.Config
	// how to handle Cmd/Entrypoint in docker
	// https://docs.docker.com/engine/reference/builder/#understand-how-cmd-and-entrypoint-interact.
	if len(cfg.Entrypoint) != 0 {
		entrypoint = cfg.Entrypoint
	} else {
		entrypoint = cfg.Cmd
	}
	return &fnrunv1alpha1.DigestAndEntrypoint{
		Digest:     hash.Hex,
		Entrypoint: entrypoint,
	}, nil
}

func podName(image, hash string) (string, error) {
	ref, err := name.ParseReference(image)
	if err != nil {
		return "", fmt.Errorf("unable to parse image reference %v: %w", image, err)
	}

	// repoName will be something like gcr.io/kpt-fn/set-namespace
	repoName := ref.Context().Name()
	parts := strings.Split(repoName, "/")
	name := strings.ReplaceAll(parts[len(parts)-1], "_", "-")
	return fmt.Sprintf("%v-%v", name, hash[:8]), nil
}
*/