package v1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	dockerclient "github.com/docker/docker/client"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/valocode/bubbly/api/common"
	"github.com/valocode/bubbly/api/core"
	"github.com/valocode/bubbly/client"
	"github.com/valocode/bubbly/env"
	"github.com/valocode/bubbly/events"

	"github.com/zclconf/go-cty/cty"
)

var _ core.Resource = (*Release)(nil)

type Release struct {
	*core.ResourceBlock
	Spec releaseSpec
}

func NewRelease(resBlock *core.ResourceBlock) *Release {
	return &Release{
		ResourceBlock: resBlock,
	}
}

func (r *Release) SpecValue() core.ResourceSpec {
	return &r.Spec
}

// Apply returns ...
func (r *Release) Apply(bCtx *env.BubblyContext, ctx *core.ResourceContext) core.ResourceOutput {
	if err := common.DecodeBodyWithInputs(bCtx, r.SpecHCL.Body, &r.Spec, ctx); err != nil {
		return core.ResourceOutput{
			ID:     r.String(),
			Status: events.ResourceRunFailure,
			Error:  fmt.Errorf(`failed to decode "%s" body spec: %s`, r.String(), err.Error()),
			Value:  cty.NilVal,
		}
	}
	// If no name was given, use the resource name as the default name
	if r.Spec.Name == "" {
		r.Spec.Name = r.ResourceName
	}

	data, err := r.Spec.Data()
	if err != nil {
		return core.ResourceOutput{
			ID:     r.ID(),
			Status: events.ResourceRunFailure,
			Error:  fmt.Errorf("failed to create data blocks: %w", err),
		}
	}

	client, err := client.New(bCtx)
	if err != nil {
		return core.ResourceOutput{
			ID:     r.ID(),
			Status: events.ResourceRunFailure,
			Error:  fmt.Errorf("error creating bubbly client: %w", err),
		}
	}

	// Marshal the release data so that it can be sent to the bubbly server
	dBytes, err := json.Marshal(data)
	if err != nil {
		return core.ResourceOutput{
			ID:     r.ID(),
			Status: events.ResourceRunFailure,
			Error:  fmt.Errorf("error marshalling release data: %w", err),
		}
	}

	// Load/Save the data blocks using the Bubbly client
	if err := client.Load(bCtx, ctx.Auth, dBytes); err != nil {
		return core.ResourceOutput{
			ID:     r.ID(),
			Status: events.ResourceRunFailure,
			Error:  fmt.Errorf("error loading release data to the bubbly server: %w", err),
		}
	}

	return core.ResourceOutput{
		ID:     r.ID(),
		Status: events.ResourceRunSuccess,
		Value:  cty.NilVal,
	}
}

type releaseSpec struct {
	Inputs        core.InputDeclarations `hcl:"input,block"`
	Name          string                 `hcl:"name,optional"`
	Version       string                 `hcl:"version,attr"`
	Product       string                 `hcl:"product,optional"`
	Project       string                 `hcl:"project,optional"`
	GitItems      []gitItem              `hcl:"git,block"`
	ArtifactItems []artifactItem         `hcl:"artifact,block"`
	Stages        []releaseStage         `hcl:"stage,block"`
}

const (
	releaseItemGitType      = "git"
	releaseItemArtifactType = "artifact"
	releaseItemReleaseType  = "release"
)

func (r *releaseSpec) Data() (core.DataBlocks, error) {
	var (
		hasProduct bool
		data       core.DataBlocks
	)
	// If Product is not empty, add it as a data block
	if r.Product != "" {
		hasProduct = true
		data = append(data, core.Data{
			TableName: "product",
			Fields: core.DataFields{
				"name": cty.StringVal(r.Product),
			},
		})
	}
	// If Project is not empty, add it as a data block
	if r.Project == "" {
		return nil, fmt.Errorf("project is required")
	}
	projectData := core.Data{
		TableName: "project",
		Fields: core.DataFields{
			"name": cty.StringVal(r.Project),
		},
	}
	if hasProduct {
		projectData.Joins = append(projectData.Joins, "product")
	}
	data = append(data, projectData)

	//
	// Create the release and release items data blocks
	//
	data = append(data, core.Data{
		TableName: "release",
		Fields: core.DataFields{
			"name":    cty.StringVal(r.Name),
			"version": cty.StringVal(r.Version),
		},
		Joins: []string{"project"},
	})
	//
	// Create the release stage data blocks
	//
	for _, stage := range r.Stages {
		data = append(data, core.Data{
			TableName: "release_stage",
			Fields: core.DataFields{
				"name": cty.StringVal(stage.Name),
			},
			Joins: []string{"release"},
		})
	}

	//
	// Create the release item and criteria data blocks
	//
	// For each release item, create the item type and the release item.
	// E.g. for git we create the commit and also the release_item that joins
	// to that specific commit.
	for _, gitItem := range r.GitItems {
		gitData, err := gitItem.Data()
		if err != nil {
			return nil, fmt.Errorf("error getting git data for repo %s: %w", gitItem.Name, err)
		}
		data = append(data, gitData...)
		data = append(data, core.Data{
			TableName: "release_item",
			Fields: core.DataFields{
				"type": cty.StringVal(releaseItemGitType),
			},
			// Join to the git commit that is created above
			Joins: []string{"commit"},
		})

		// Criteria are specific to a stage and also to a release item.
		// So we need to create it for each release item.
		data = append(data, r.criteriaData()...)
	}
	for _, artifactItem := range r.ArtifactItems {
		artifactData, err := artifactItem.Data()
		if err != nil {
			return nil, fmt.Errorf("error getting artifact data for %s: %w", artifactItem.Name, err)
		}
		data = append(data, artifactData...)
		data = append(data, core.Data{
			TableName: "release_item",
			Fields: core.DataFields{
				"type": cty.StringVal(releaseItemArtifactType),
			},
			// Join to the git commit that is created above
			Joins: []string{"artifact"},
		})

		// Criteria are specific to a stage and also to a release item.
		// So we need to create it for each release item.
		data = append(data, r.criteriaData()...)
	}

	return data, nil
}

// criteriaData creates the data blocks for each criteria, which are specific
// to a release_stage and release_item
// TODO: because no for_each or similar behaviour exists for data blocks, these
// criteria data blocks get duplicated for each release_item.
// This may not be such a problem, but leaving this comment here for potential
// future optimization gains
func (r *releaseSpec) criteriaData() core.DataBlocks {
	var data core.DataBlocks
	for _, stage := range r.Stages {
		data = append(data, core.Data{
			TableName: "release_stage",
			Fields: core.DataFields{
				"name": cty.StringVal(stage.Name),
			},
			Joins: []string{"release"},
			// We only want to reference and make sure we do not re-create
			// a stage
			Policy: core.ReferencePolicy,
		})
		for _, criteria := range stage.Crterion {
			data = append(data, core.Data{
				TableName: "release_criteria",
				Fields: core.DataFields{
					"entry_name": cty.StringVal(criteria.Name),
				},
				Joins: []string{"release_stage", "release_item"},
			})
		}
	}
	return data
}

type gitItem struct {
	Name string `hcl:"name,attr"`
	Repo string `hcl:"repo,attr"`
}

func (g *gitItem) Data() (core.DataBlocks, error) {
	repo, err := git.PlainOpen(g.Repo)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository %s: %w", g.Repo, err)
	}
	ref, err := repo.Head()
	if err != nil {
		return nil, fmt.Errorf("failed to get HEAD of repo %s: %w", g.Repo, err)
	}
	commit, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return nil, fmt.Errorf("failed to get commit from HEAD %s: %w", ref.Hash().String(), err)
	}

	repoData := core.Data{
		TableName: "repo",
		Fields: core.DataFields{
			"name": cty.StringVal(g.Name),
		},
		Joins: []string{"project"},
	}
	// If HEAD is not detached, then we can add the branch name to the git data
	// block
	if ref.Name().IsBranch() {
		repoData.Data = append(repoData.Data, core.Data{
			TableName: "branch",
			Fields: core.DataFields{
				"name": cty.StringVal(ref.Name().Short()),
			},
		})
	}

	commitData := core.Data{
		TableName: "commit",
		Fields: core.DataFields{
			"id":   cty.StringVal(ref.Hash().String()),
			"time": cty.StringVal(commit.Author.When.String()),
		},
		Joins: []string{"repo"},
	}
	// Add the tag, if there is one for this commit, to the commit data
	tagrefs, err := repo.Tags()
	if err != nil {
		return nil, fmt.Errorf("failed to read tags from repo %s, error %w", g.Repo, err)
	}
	// Ignore the returned error, as it shouldn't be triggered
	tagrefs.ForEach(func(t *plumbing.Reference) error {
		// If the tag ref is for the same commit, we have a match and so add the
		// tag to the commit data
		if ref.Hash() == t.Hash() {
			commitData.Fields["tag"] = cty.StringVal(t.Name().Short())
		}
		return nil
	})
	// commitData.Fields["tag"] = cty.StringVal("lala")
	// If HEAD is not detached, also join the commit to the branch
	if ref.Name().IsBranch() {
		commitData.Joins = append(commitData.Joins, "branch")
	}

	return core.DataBlocks{
		repoData, commitData,
	}, nil
}

const (
	artifactTypeDiv      = "://"
	artifactFilePrefix   = "file" + artifactTypeDiv
	artifactDockerPrefix = "docker" + artifactTypeDiv
)

type artifactItem struct {
	Name     string `hcl:"name,attr"`
	Location string `hcl:"location,attr"`
}

func (a *artifactItem) Data() (core.DataBlocks, error) {
	var (
		sha256 string
		err    error
	)
	switch {
	case strings.HasPrefix(a.Location, artifactFilePrefix):
		sha256, err = a.sha256SumFile()
	case strings.HasPrefix(a.Location, artifactDockerPrefix):
		sha256, err = a.sha256SumDocker()
	default:
		// Check if there was supposed to be a type given before the ://
		// If so, it's an unknown type and we should error
		if strings.Contains(a.Location, artifactTypeDiv) {
			// Get the substring up to and including ://
			typeStr := a.Location[0 : strings.Index(a.Location, artifactTypeDiv)+3]
			return nil, fmt.Errorf("unkown artifact type: %s", typeStr)
		}
		// Treat as a file by default
		sha256, err = a.sha256SumFile()
	}
	if err != nil {
		return nil, fmt.Errorf("error calculating sha256 sum of %s: %w", a.Location, err)
	}

	return core.DataBlocks{
		core.Data{
			TableName: "artifact",
			Fields: core.DataFields{
				"name":     cty.StringVal(a.Name),
				"location": cty.StringVal(a.Location),
				"sha256":   cty.StringVal(sha256),
			},
		},
	}, nil
}

func (a *artifactItem) sha256SumFile() (string, error) {
	loc := strings.TrimPrefix(a.Location, artifactFilePrefix)
	f, err := os.Open(loc)
	if err != nil {
		return "", fmt.Errorf("error opening artifact file %s: %w", loc, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("error reading artifact file %s: %w", loc, err)
	}
	// Sum comes back as hex, so need to encode that
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (a *artifactItem) sha256SumDocker() (string, error) {
	loc := strings.TrimPrefix(a.Location, artifactDockerPrefix)
	cli, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
	if err != nil {
		return "", fmt.Errorf("error creating docker client: %w", err)
	}
	t, _, err := cli.ImageInspectWithRaw(context.TODO(), loc)
	if err != nil {
		return "", fmt.Errorf("error inspecting image %s: %w", loc, err)
	}

	return strings.TrimPrefix(t.ID, "sha256:"), nil
}

type releaseStage struct {
	Name     string            `hcl:",label"`
	Crterion []releaseCriteria `hcl:"criteria,block"`
}

type releaseCriteria struct {
	Name string `hcl:",label"`
}
