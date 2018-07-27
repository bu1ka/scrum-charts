package jira

import (
	"net/http"
	"encoding/json"
    "net/url"
    "io/ioutil"
    "strconv"
	"log"
    "time"
    "reflect"

	"github.com/JaverSingleton/scrum-charts/config"
)

func GetIssues(config config.Config, credentials config.Credentials) ([]Issue, error) {
	var jql string
	if (config.Query != "") {
		jql = config.Query
	} else {
		jql = "Sprint = " + strconv.Itoa(config.Code) + " " +
                "AND type != Epic " +
                "AND (resolutiondate is EMPTY OR resolutiondate >= \"" + config.StartDate + "\")" +
                "AND (\"Feature Team\" is EMPTY OR \"Feature Team\" = " + config.Team + ")"
	}
    log.Println("Search:", jql)
	return Search(config, credentials, jql)
}

func Search(config config.Config, credentials config.Credentials, jql string) ([]Issue, error) { 
	var Url *url.URL
    Url, err := url.Parse("https://jr.avito.ru")
    if err != nil {
    	return []Issue {}, err
    }

    Url.Path += "/rest/api/2/search"
    parameters := url.Values{}
    parameters.Add("jql", jql)
    parameters.Add("maxResults", "500")
    Url.RawQuery = parameters.Encode()

    log.Println("Create Request:", Url.String())
	req, err := http.NewRequest("GET", Url.String(), nil)
	if (err != nil) {
    	return []Issue {}, err
	}
    req.SetBasicAuth(credentials.Login, credentials.Password)
	client := &http.Client {}
    log.Println("Do Request:", Url.String())
	response, err := client.Do(req)
    if err != nil {
    	return []Issue {}, err
    }
    defer response.Body.Close()
    log.Println("Read Body")
    contents, err := ioutil.ReadAll(response.Body)
    if err != nil {
    	return []Issue {}, err
    }
    var search = JiraSearch {}
    log.Println("Umarshal JSON:", string(contents[:]))
	if err = json.Unmarshal(contents, &search); err != nil {
		return []Issue {}, err
	}
	stories := collectStories(search.Issues)
	issues:= make([]Issue, len(search.Issues))
	issuesMap := make(map[string]Issue)
	for index, element := range search.Issues {
		key, issue := convertJiraIssue(stories, element)
		issues[index] = issue
		issuesMap[key] = issue
	}
    log.Println("Start Children processing")
	for index, issue := range issues {
		for _, childIssueKey := range issue.Subtasks {
    		log.Println("Children key:", issue.Title, childIssueKey)
			if childIssue, ok := issuesMap[childIssueKey]; ok {    
				issues[index].ChildrenStories += childIssue.StoryPoints
				issues[index].IsProgress = issues[index].IsProgress || childIssue.IsProgress
			}
		}
    	log.Println("Children stories:", issue.Title, issue.ChildrenStories)
	}
    log.Println("Finish Children processing")
    log.Println(issues)
	return issues, nil
}

func collectStories(issues []JiraIssue) map[string]string {
	stories := make(map[string]string)
	for _, issue := range issues {
		if (issue.Fields.Issuetype.Name == "Story") {
			stories[issue.Key] = issue.Fields.Summary
		}
	}
	return stories
}

func convertJiraIssue(stories map[string]string, jiraIssue JiraIssue) (string, Issue) {
    log.Println("Issue processing:", jiraIssue)
	resolutionDate := convertDate(jiraIssue.Fields.Resolutiondate)
	var blocks []string
    log.Println("Start Blocks processing")
	for _, element := range jiraIssue.Fields.Issuelinks {
		if (element.Type.Name == "Blocks") {
			if storyTitle, ok := stories[element.OutwardIssue.Key]; ok {    
				blocks = append(blocks, storyTitle)
			}
		}
	}
	platforms:= make([]string, len(jiraIssue.Fields.Components))
	for index, component := range jiraIssue.Fields.Components {
		platforms[index] = component.Name
	}
    log.Println("Finish Blocks processing")
	subtasks:= make([]string, len(jiraIssue.Fields.Subtasks))
	for index, subtask := range jiraIssue.Fields.Subtasks {
		subtasks[index] = subtask.Key
	}
	var progressValues []string = []string{
		"Waiting for release", 
		"In Progress", 
		"In test", 
		"In Review", 
		"QA Progress",
	}
	isProgress, _ := contains(jiraIssue.Fields.Status.Name, progressValues)
	return jiraIssue.Key, Issue {
		StoryPoints: jiraIssue.Fields.Customfield_10212,
		CloseDate: resolutionDate,
		Title: jiraIssue.Fields.Summary,
		Parents: blocks,
		Platforms: platforms,
		Subtasks: subtasks,
		IsProgress: isProgress,
	}
}

func contains(v interface{}, in interface{}) (ok bool, i int) {
    val := reflect.Indirect(reflect.ValueOf(in))
    switch val.Kind() {
    case reflect.Slice, reflect.Array:
        for ; i < val.Len(); i++ {
            if ok = v == val.Index(i).Interface(); ok {
                return
            }
        }
    }
    return
}

func convertDate(date string) string {
	time, err := time.Parse("2006-01-02T15:04:05-0700", date)
    log.Println(date, time)
	if err != nil {
	    return ""
	}
	return time.Format("2006-01-02")
}

type Issue struct {
	StoryPoints float64 `json:"storyPoints"`
	CloseDate string `json:"closeDate"`
    Title string `json:"title"`
    Parents []string `json:"parents"`
    Platforms []string `json:"platforms"`
    ChildrenStories float64 `json:"childrenStories"`
    Subtasks []string `json:"subtasks"`
    IsProgress bool `json:"isProgress"`
}

type JiraIssue struct {
    Id string `json:"id"`
    Key string `json:"key"`
    Fields JiraFields `json:"fields"`
}

type JiraFields struct {
    Customfield_10212 float64 `json:"customfield_10212"`
    Resolutiondate string `json:"resolutiondate"`
    Summary string `json:"summary"`
    Issuetype JiraType `json:"issuetype"`
    Status JiraStatus `json:"status"`
    Issuelinks []JiraLink `json:"issuelinks"`
    Components []JiraComponent `json:"components"`
    Subtasks []JiraIssue `json:"subtasks"`
}

type JiraSearch struct {
    Expand string `json:"expand"`
    Issues []JiraIssue `json:"issues"`
    MaxResults int `json:"maxResults"`
    StartAt int `json:"startAt"`
    Total int `json:"total"`
}

type JiraType struct {
    Id string `json:"id"`
    Name string `json:"name"`
}

type JiraLink struct {
    Id string `json:"id"`
    Type JiraType `json:"type"`
    OutwardIssue JiraOutward `json:"outwardIssue"`
}

type JiraOutward struct {
    Id string `json:"id"`
    Key string `json:"key"`
}

type JiraComponent struct {
    Id string `json:"id"`
    Name string `json:"name"`
}

type JiraStatus struct {
    Id string `json:"id"`
    Name string `json:"name"`
}