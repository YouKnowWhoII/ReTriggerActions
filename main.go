package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

// GitHubToken is your GitHub Personal Access Token
const GitHubToken = ""

// Organization is the name of your GitHub organization
const Organization = ""

// BaseURL is the base URL for the GitHub API
const BaseURL = "https://api.github.com"

// Repository represents the structure of a GitHub repository
type Repository struct {
	Name string `json:"name"`
}

// WorkflowRun represents a workflow run in a repository
type WorkflowRun struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
	Name   string `json:"name"`
}

// AuthHeader generates the authorization header
func AuthHeader() map[string]string {
	return map[string]string{
		"Authorization": fmt.Sprintf("Bearer %s", GitHubToken),
		"Accept":        "application/vnd.github.v3+json",
	}
}

// makeRequest sends an HTTP request to the GitHub API
func makeRequest(method, url string, body []byte) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}

	for key, value := range AuthHeader() {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}

	return ioutil.ReadAll(resp.Body)
}

// getRepositories fetches all repositories in the organization
func getRepositories() ([]Repository, error) {
	var repos []Repository
	page := 1
	for {
		url := fmt.Sprintf("%s/orgs/%s/repos?per_page=100&page=%d", BaseURL, Organization, page)
		data, err := makeRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		var batch []Repository
		if err := json.Unmarshal(data, &batch); err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			break
		}

		repos = append(repos, batch...)
		page++
	}

	return repos, nil
}

// getLatestWorkflowRun fetches the latest workflow run for a repository
func getLatestWorkflowRun(repoName string) (WorkflowRun, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs?per_page=1", BaseURL, Organization, repoName)
	data, err := makeRequest("GET", url, nil)
	if err != nil {
		return WorkflowRun{}, err
	}

	var response struct {
		WorkflowRuns []WorkflowRun `json:"workflow_runs"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return WorkflowRun{}, err
	}

	if len(response.WorkflowRuns) == 0 {
		return WorkflowRun{}, fmt.Errorf("no workflow runs found for repository: %s", repoName)
	}

	return response.WorkflowRuns[0], nil
}

// rerunWorkflow triggers a re-run of a workflow run
func rerunWorkflow(repoName string, runID int) error {
	url := fmt.Sprintf("%s/repos/%s/%s/actions/runs/%d/rerun", BaseURL, Organization, repoName, runID)

	client := &http.Client{}
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %v", err)
	}

	for key, value := range AuthHeader() {
		req.Header.Set(key, value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 {
		responseBody, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s\nResponse Body: %s", resp.StatusCode, http.StatusText(resp.StatusCode), string(responseBody))
	}

	return nil
}

// main orchestrates fetching repositories, workflow runs, and re-triggering them
func main() {
	repos, err := getRepositories()
	if err != nil {
		fmt.Printf("Error fetching repositories: %v\n", err)
		return
	}

	for _, repo := range repos {
		fmt.Printf("Processing repository: %s\n", repo.Name)

		latestRun, err := getLatestWorkflowRun(repo.Name)
		if err != nil {
			fmt.Printf("Error fetching latest workflow run for %s: %v\n", repo.Name, err)
			continue
		}

		fmt.Printf("Re-running workflow: %s (Run ID: %d)\n", latestRun.Name, latestRun.ID)
		err = rerunWorkflow(repo.Name, latestRun.ID)
		if err != nil {
			fmt.Printf("Failed to re-run workflow for %s: %v\n", repo.Name, err)
		} else {
			fmt.Printf("Successfully re-ran workflow for %s\n", repo.Name)
		}
	}
}
