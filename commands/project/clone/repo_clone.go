package clone

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/profclems/glab/internal/glinstance"

	"github.com/profclems/glab/commands/cmdutils"
	"github.com/profclems/glab/internal/git"
	"github.com/profclems/glab/internal/glrepo"
	"github.com/profclems/glab/pkg/api"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"
	"github.com/xanzy/go-gitlab"
)

func NewCmdClone(f *cmdutils.Factory) *cobra.Command {
	var repoCloneCmd = &cobra.Command{
		Use:   "clone <command> [flags]",
		Short: `Clone a Gitlab repository/project`,
		Example: heredoc.Doc(`
	$ glab repo clone profclems/glab
	$ glab repo clone https://gitlab.com/profclems/glab
	$ glab repo clone profclems/glab mydirectory  # Clones repo into mydirectory
	$ glab repo clone glab   # clones repo glab for current user 
	$ glab repo clone 4356677   # finds the project by the ID provided and clones it
	`),
		Long: heredoc.Doc(`
	Clone supports these shorthands
	- repo
	- namespace/repo
	- namespace/group/repo
	- project ID
	`),
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				project   *gitlab.Project = nil
				host      string
				err       error
				apiClient *gitlab.Client
			)

			baseRepo, err := f.BaseRepo()
			if err != nil {
				host = glinstance.Default()
			} else {
				host = baseRepo.RepoHost()
			}

			cfg, _ := f.Config()
			client, _ := api.NewClientWithCfg(host, cfg, false)
			apiClient = client.Lab()

			repo := args[0]
			u, err := api.CurrentUser(apiClient)
			if err != nil {
				return err
			}

			protocol, _ := cfg.Get(host, "git_protocol")

			remoteArgs := &glrepo.RemoteArgs{
				Protocol: protocol,
				Token:    client.Token(),
				Url:      host,
				Username: u.Username,
			}

			if !git.IsValidURL(repo) {
				// Assuming that repo is a project ID if it is an integer
				if _, err := strconv.ParseInt(repo, 10, 64); err != nil {
					// Assuming that "/" in the project name means its owned by an organisation
					if !strings.Contains(repo, "/") {
						repo = fmt.Sprintf("%s/%s", u.Username, repo)
					}
				}
				project, err = api.GetProject(apiClient, repo)
				if err != nil {
					return err
				}
				repo, err = glrepo.RemoteURL(project, remoteArgs)
				if err != nil {
					return err
				}
			} else if !strings.HasSuffix(repo, ".git") {
				repo += ".git"
			}
			_, err = git.RunClone(repo, args[1:])
			if err != nil {
				return err
			}
			// Cloned project was a fork belonging to the user; user is
			// treating fork's ssh url as origin. Add upstream as remote pointing
			// to forked repo's ssh url
			if project != nil {
				if project.ForkedFromProject != nil &&
					strings.Contains(project.PathWithNamespace, u.Username) {
					var dir string
					if len(args) > 1 {
						dir = args[1]
					} else {
						dir = "./" + project.Path
					}
					fProject, err := api.GetProject(apiClient, project.ForkedFromProject.PathWithNamespace)
					if err != nil {
						return err
					}
					repoURL, err := glrepo.RemoteURL(fProject, &glrepo.RemoteArgs{})
					if err != nil {
						return err
					}
					err = git.AddUpstreamRemote(repoURL, dir)
					if err != nil {
						return err
					}
				}
			}
			return nil
		},
	}

	return repoCloneCmd
}
