package cmd

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tcnksm/go-input"
)

const defaultRepository = "cybozu/csa"

var draftOpts struct {
	title string
	repo  string
	issue int
}

func taskArguments(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	if len(args) != 1 {
		return errors.New("too many arguments")
	}

	issueNum := args[0]
	parts := strings.Split(args[0], "#")
	if len(parts) > 2 {
		return errors.New("too many '#' in issue number")
	} else if len(parts) == 2 {
		draftOpts.repo = parts[0]
		issueNum = parts[1]
	}

	num, err := strconv.Atoi(issueNum)
	if err != nil {
		return err
	}
	draftOpts.issue = num

	return nil
}

var draftCmd = &cobra.Command{
	Use:   "draft [ISSUE]",
	Short: "create a draft pull request for the current branch",
	Long: `Create a draft pull request for the current branch.

If ISSUE is given, this command connects the new pull request with the issue.
The ISSUE can be specified in one of the following formats.
  - <issue number>
  - <owner>/<repo>#<issue number>
  - https://github.com/<owner>/<repo>#<issue number>
  - git@github.com:<owner>/<repo>#<issue number>`,

	Args: taskArguments,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDraftCmd(cmd, args, true)
	},
}

func init() {
	draftCmd.Flags().StringVar(&draftOpts.title, "title", "", "title of the pull request")
	rootCmd.AddCommand(draftCmd)
}

func githubClient(ctx context.Context) (GitHubClient, error) {
	token := config.GithubToken
	return NewGitHubClient(ctx, token), nil
}

func runDraftCmd(cmd *cobra.Command, args []string, draft bool) error {

	branch, err := currentBranch()
	if err != nil {
		return err
	}
	defBranch, err := defaultBranch()
	if err != nil {
		return err
	}
	if branch == defBranch {
		return fmt.Errorf("direct push to %s is prohibited", defBranch)
	}

	_, firstSummary, firstBody, err := firstUnmerged(defBranch)
	if err != nil {
		return err
	}

	if ok, err := confirmUncommitted(); err != nil {
		return err
	} else if !ok {
		return nil
	}

	ctx := context.Background()
	gc, err := githubClient(ctx)
	if err != nil {
		return err
	}

	curRepo, err := getCurrentRepo(ctx, gc)
	if err != nil {
		return err
	}

	repo := defaultRepository
	if draftOpts.repo != "" {
		repo = draftOpts.repo
	}
	issueRepo, err := getRepo(ctx, gc, repo)
	if err != nil {
		return err
	}

	if draftOpts.issue != 0 {
		if ok, err := confirmIssue(ctx, gc, issueRepo, draftOpts.issue); err != nil {
			return err
		} else if !ok {
			return nil
		}
	}

	if err := git("push", "-u", "origin", branch+":"+branch); err != nil {
		return err
	}

	title := draftOpts.title
	if title == "" {
		title = firstSummary
	}

	message := firstBody
	if draftOpts.issue != 0 {
		message = insertIssueLink(message, issueRepo.Owner, issueRepo.Name, draftOpts.issue)
	}

	_, err = createPR(ctx, gc, curRepo, defBranch, branch, title, message, draft)
	if err != nil {
		return err
	}

	return nil
}

// insertIssueLink inserts issue link before Signed-off-by: into a commit message.
func insertIssueLink(message string, issueRepoOwner, issueRepoName string, issueNum int) string {
	issueLink := fmt.Sprintf("issue: %s/%s#%d", issueRepoOwner, issueRepoName, issueNum)
	signedOffBy := "Signed-off-by:"

	if strings.Contains(message, issueLink) {
		return message
	}

	messageFragment := message
	var signedOfByFragment string
	if index := strings.Index(message, signedOffBy); index != -1 {
		messageFragment = message[0:index]
		signedOfByFragment = message[index:]
	}

	generatedMessage := messageFragment
	if messageFragment != "" && !strings.HasSuffix(messageFragment, "\n") {
		generatedMessage += "\n"
	}
	generatedMessage += issueLink
	if signedOfByFragment != "" {
		generatedMessage += "\n" + signedOfByFragment
	}

	return generatedMessage
}

func askYorN(query string) (bool, error) {
	ans, err := input.DefaultUI().Ask(query+" [y/N]", &input.Options{
		Default:     "N",
		HideDefault: true,
		HideOrder:   true,
	})
	if err != nil {
		return false, err
	}
	switch ans {
	case "y", "Y", "yes", "YES":
		return true, nil
	}
	return false, nil
}

func confirmUncommitted() (bool, error) {
	ok, err := checkUncommittedFiles()
	if err != nil {
		return false, err
	}
	if ok {
		return true, nil
	}

	fmt.Println("WARNING: you have uncommitted files.")
	return askYorN("Continue?")
}

func confirmIssue(ctx context.Context, gc GitHubClient, repo *GitHubRepository, issue int) (bool, error) {
	title, err := gc.GetIssueTitle(ctx, repo, issue)
	if err != nil {
		return false, err
	}

	fmt.Printf("%s/%s#%d: %s\n", repo.Owner, repo.Name, issue, title)
	return askYorN("Is this ok?")
}

func getRepo(ctx context.Context, gc GitHubClient, repo string) (*GitHubRepository, error) {
	owner, name, err := ExtractGitHubRepositoryName(repo)
	if err != nil {
		return nil, err
	}

	return gc.GetRepository(ctx, owner, name)
}

func getCurrentRepo(ctx context.Context, gc GitHubClient) (*GitHubRepository, error) {
	origin, err := originURL()
	if err != nil {
		return nil, err
	}
	return getRepo(ctx, gc, origin)
}

// Create a new pull request and add assignee to the pull request.
func createPR(ctx context.Context, gc GitHubClient, repo *GitHubRepository, defBranch, branch, title, body string, draft bool) (*PullRequest, error) {
	pr, err := gc.CreatePullRequest(ctx, repo.ID, defBranch, branch, title, body, draft)
	if err != nil {
		return nil, err
	}
	fmt.Println("\nCreated a pull request:", pr.Permalink)

	assignee, err := gc.GetViewer(ctx)
	if err != nil {
		return nil, err
	}

	err = gc.AddAssigneeToPullRequest(ctx, assignee.ID, pr.ID)
	if err != nil {
		return nil, err
	}

	return pr, nil
}
