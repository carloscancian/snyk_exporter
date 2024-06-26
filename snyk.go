package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httputil"

	"github.com/prometheus/common/log"
)

type client struct {
	httpClient *http.Client
	token      string
	baseURL    string
}

func (c *client) getOrganizations() (orgsResponse, error) {
	log.Debugf("Start finding organizations")
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/rest/orgs", c.baseURL), nil)
	if err != nil {
		return orgsResponse{}, err
	}
	response, err := c.do(req)
	if err != nil {
		return orgsResponse{}, err
	}
	var orgs orgsResponse
	err = json.NewDecoder(response.Body).Decode(&orgs)
	if err != nil {
		return orgsResponse{}, err
	}
	log.Debugf("Done finding organizations, found: %d", len(orgs.Orgs))
	return orgs, nil
}

func (c *client) getProjects(organizationID string) (projectsResponse, error) {
	log.Debugf("Start finding projects for: %s", organizationID)
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/rest/orgs/%s/projects", c.baseURL, organizationID), nil)
	if err != nil {
		return projectsResponse{}, err
	}
	response, err := c.do(req)
	if err != nil {
		return projectsResponse{}, err
	}
	var projectsResponseObject projectsResponse
	err = json.NewDecoder(response.Body).Decode(&projectsResponseObject)
	if err != nil {
		return projectsResponse{}, err
	}
	var projects []project
	projects = append(projects, projectsResponseObject.Projects...)

	var nextLink string = projectsResponseObject.Links.Next
	// Loop over subsequent pages if there is a 'next' link
	for nextLink != "" {
		log.Debugf("More projects to be found, currently: %d", len(projects))
		req, err := http.NewRequest(http.MethodGet, c.baseURL+nextLink, nil)
		if err != nil {
			return projectsResponse{}, err
		}
		response, err := c.do(req)
		if err != nil {
			return projectsResponse{}, err
		}
		defer response.Body.Close()

		err = json.NewDecoder(response.Body).Decode(&projectsResponseObject)
		if err != nil {
			return projectsResponse{}, err
		}

		projects = append(projects, projectsResponseObject.Projects...)

		if nextLink == projectsResponseObject.Links.Next {
			log.Debugf("No more new link, stopping")
			break
		}
		nextLink = projectsResponseObject.Links.Next
	}
	log.Debugf("Done finding projects for: %s, found: %d", organizationID, len(projects))
	return projectsResponse{projects, links{}}, nil
}

func (c *client) getIssues(organizationID, projectID string) (issuesResponse, error) {
	log.Debugf("Start finding issues for: %s", projectID)
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/rest/orgs/%s/issues", c.baseURL, organizationID), nil)
	if err != nil {
		return issuesResponse{}, err
	}
	// Find any issues that has a relation to this project ID
	query := req.URL.Query()
	query.Set("scan_item.id", projectID)
	query.Set("scan_item.type", "project")
	req.URL.RawQuery = query.Encode()

	response, err := c.do(req)
	if err != nil {
		return issuesResponse{}, err
	}
	defer response.Body.Close()

	var issuesResponseObject issuesResponse
	err = json.NewDecoder(response.Body).Decode(&issuesResponseObject)
	if err != nil {
		return issuesResponse{}, err
	}

	var issues []issue
	issues = append(issues, issuesResponseObject.Issues...)

	var nextLink string = issuesResponseObject.Links.Next
	// Loop over subsequent pages if there is a 'next' link
	for nextLink != "" {
		log.Debugf("More issues to be found, currently: %d", len(issues))
		req, err := http.NewRequest(http.MethodGet, c.baseURL+nextLink, nil)
		// Find any issues that has a relation to this project ID
		query := req.URL.Query()
		query.Set("scan_item.id", projectID)
		query.Set("scan_item.type", "project")
		req.URL.RawQuery = query.Encode()

		if err != nil {
			return issuesResponse{}, err
		}
		response, err := c.do(req)
		if err != nil {
			return issuesResponse{}, err
		}
		defer response.Body.Close()

		err = json.NewDecoder(response.Body).Decode(&issuesResponseObject)
		if err != nil {
			return issuesResponse{}, err
		}

		issues = append(issues, issuesResponseObject.Issues...)

		if nextLink == issuesResponseObject.Links.Next {
			log.Debugf("No more new link, stopping")
			break
		}
		nextLink = issuesResponseObject.Links.Next
	}

	log.Debugf("Done finding issues for: %s, found: %d", projectID, len(issues))
	return issuesResponse{issues, links{}}, nil
}

func (c *client) do(req *http.Request) (*http.Response, error) {
	req.Header.Add("authorization", fmt.Sprintf("TOKEN %s", c.token))

	query := req.URL.Query()
	query.Set("version", "2024-01-23")
	query.Set("limit", "100")
	req.URL.RawQuery = query.Encode()

	log.Debugf("Running request to URL: %v", req.URL)
	response, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			log.Errorf("read body failed: %v", err)
			body = []byte("failed to read body")
		}
		requestDump, err := httputil.DumpRequestOut(req, true)
		if err != nil {
			log.Debugf("Failed to dump request for logging")
		} else {
			log.Debugf("Failed request dump: %s", requestDump)
		}
		return nil, fmt.Errorf("request not OK: %s: url: %s body: %s", response.Status, req.URL, body)
	}
	return response, nil
}

type links struct {
	Next string `json:"next,omitempty"`
}

type orgsResponse struct {
	Orgs  []org `json:"data,omitempty"`
	Links links `json:"links,omitempty"`
}

type org struct {
	ID         string        `json:"id,omitempty"`
	Type       string        `json:"type,omitempty"`
	Attributes orgAttributes `json:"attributes,omitempty"`
}

type orgAttributes struct {
	GroupID string `json:"group_id,omitempty"`
	Name    string `json:"name,omitempty"`
}

type projectsResponse struct {
	Projects []project `json:"data,omitempty"`
	Links    links     `json:"links,omitempty"`
}

type project struct {
	ID         string            `json:"id,omitempty"`
	Type       string            `json:"type,omitempty"`
	Attributes projectAttributes `json:"attributes,omitempty"`
}

type projectAttributes struct {
	Name string `json:"name,omitempty"`
}

type issuesResponse struct {
	Issues []issue `json:"data,omitempty"`
	Links  links   `json:"links,omitempty"`
}

type issue struct {
	Attributes  issueAttributes `json:"attributes,omitempty"`
	Coordinates coordinates     `json:"coordinates,omitempty"`
}

type issueAttributes struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Title    string `json:"title,omitempty"`
	Severity string `json:"effective_severity_level,omitempty"`
	Ignored  bool   `json:"ignored"`
}

type coordinates struct {
	Upgradeable bool   `json:"is_upgradable"`
	Patchable   bool   `json:"is_patchable"`
	Type        string `json:"type"`
}

type license struct{}
