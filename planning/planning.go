package planning

import (
    "strconv"
	"log"
	"time"
	"strings"
    "reflect"

	"github.com/JaverSingleton/scrum-charts/jira"
	"github.com/JaverSingleton/scrum-charts/config"
)

func GetPlanningInfo(manager *jira.JobManager, team *config.FeatureTeam) (PlanningInfo, error) {
	if (team == nil) {
		return PlanningInfo {}, nil
	}

	lostIssues, plannedIssues, _ := findLostAndPlannedIssues(manager, "")

	users := make(map[string]User)

	for _, teamUser := range team.Users {
		users[teamUser.Name] = createUser(teamUser, plannedIssues, lostIssues)
	}
    
    return PlanningInfo { 
    	Users: users,
    	MaxStoryPoints: team.SpPerDay * float64(calculateDatesDelta(manager.Config.StartDate, manager.Config.FinishDate) - len(manager.Config.Weekend) - 2),
    }, nil
}

func findLostAndPlannedIssues(manager *jira.JobManager, teamName string) (lostIssues []Issue, plannedIssues []Issue, requestDate time.Time) {
	plannedJql := "Sprint = " + strconv.Itoa(manager.Config.Code) + " AND " + 
		"type != Story"
	if (teamName != "") {
		plannedJql += " AND \"Feature teams\"  = " + teamName
	}
	plannedChannel := make(chan Issues)
	go find(manager, plannedJql, plannedChannel)

	lostJql := "Sprint = " + strconv.Itoa(manager.Config.PrevCode) + " AND " + 
		"NOT Sprint in openSprints() AND " + 
		"NOT Sprint in futureSprints() AND " + 
		"type != Epic AND type != Story" + " AND " +
		"resolutiondate is EMPTY"
	if (teamName != "") {
		lostJql += " AND \"Feature teams\"  = " + teamName
	}
	lostChannel := make(chan Issues)
	go find(manager, lostJql, lostChannel)

	lostIssuesResult, plannedIssuesResult := <- lostChannel, <- plannedChannel
	lostIssues = lostIssuesResult.Issues
	plannedIssues = plannedIssuesResult.Issues
	requestDate = plannedIssuesResult.RequestDate
	return
}

func find(manager *jira.JobManager, jql string, issues chan<- Issues) {
	search := make(chan jira.Search)
	go manager.AddJob(jql, search)
	issues <- convert(<-search)
}

func createUser(user config.User, plannedIssues []Issue, lostIssues []Issue) User {
	var userPlannedIssues []Issue = make([]Issue, 0)
	for _, issue := range plannedIssues {
		if (issue.Assignee == user.Name) {
			userPlannedIssues = append(userPlannedIssues, issue)
		}
	}
	var userLostIssues []Issue = make([]Issue, 0)
	for _, issue := range lostIssues {
		if (issue.Assignee == user.Name) {
			userLostIssues = append(userLostIssues, issue)
		}
	}
	return User {
		Name: user.Name,
		PlannedIssues: userPlannedIssues,
		LostIssues: userLostIssues,
	}
}

func convert(jiraSearch jira.Search) Issues {
    log.Println("Issues processing - Start: Count = ", len(jiraSearch.Issues))
	var result []Issue = make([]Issue, 0)
	issues := make(map[string]Issue)
	for _, jiraIssue := range jiraSearch.Issues {
		issues[jiraIssue.Key] = convertIssue(jiraIssue)
	}
	for _, jiraIssue := range jiraSearch.Issues {
		if issue, ok := issues[jiraIssue.Key]; ok {
			if (issue.Type == "QA" || issue.Type == "TestCase") {
				issue.Development = findDevelopmentIssue(issues, jiraIssue)
				issue.Epic = findEpicIssue(issues, jiraIssue)
			} else if (issue.Type != "Epic"){
				issue.QA = findTestIssue(issues, jiraIssue)
				issue.TestCases = findTestCassesIssue(issues, jiraIssue)
				issue.Epic = findEpicIssue(issues, jiraIssue)
			}
			result = append(result, issue)
		}
	}
    log.Println("Issues processing - Completed: Count = ", len(jiraSearch.Issues))
	return Issues {
		Issues: result,
		RequestDate: jiraSearch.RequestDate,
	}
}

func convertIssue(jiraIssue jira.Issue) Issue {
	return Issue { 
		Key: jiraIssue.Key,
		Name: jiraIssue.Fields.Summary,
		StoryPoints: jiraIssue.Fields.Customfield_10212,
		Type: jiraIssue.Fields.Issuetype.Name,
		Assignee: jiraIssue.Fields.Assignee.Name,
		Platform: jiraIssue.Fields.Components[0].Name,
		Uri: createUri(jiraIssue.Key),
		IsResolved: jiraIssue.Fields.Status.Category.Key == "done",
		IsEasy: contains("Easy", jiraIssue.Fields.Labels),
	}
}

func findTestIssue(issues map[string]Issue, targetIssue jira.Issue) *Issue {
	qaIssues := findQaIssue(issues, targetIssue)
	for _, qaIssue := range qaIssues {
		if !hasTestsCassesSubstring(qaIssue.Name) {    
			return &qaIssue
		} 
	}

	return nil
}

func findTestCassesIssue(issues map[string]Issue, targetIssue jira.Issue) *Issue {
	qaIssues := findQaIssue(issues, targetIssue)
	for _, qaIssue := range qaIssues {
		if hasTestsCassesSubstring(qaIssue.Name) || qaIssue.Type == "TestCase"  {    
			return &qaIssue
		} 
	}

	return nil
}

func findEpicIssue(issues map[string]Issue, targetIssue jira.Issue) *Issue {
	if (targetIssue.Fields.Epic == "") {
		return nil
	}
	if epic, ok := issues[targetIssue.Fields.Epic]; ok {  
		return &epic
	} else {
		return &Issue {
			Key: targetIssue.Fields.Epic,
			OutSprint: true,
			Type: "Epic",
			Uri: createUri(targetIssue.Fields.Epic),
			IsResolved: false,
		}
	}
}

func findDevelopmentIssue(issues map[string]Issue, targetIssue jira.Issue) *Issue {
	developmentIssues := findDevelopmentIssues(issues, targetIssue)
	if (len(developmentIssues) > 0) {
		return &developmentIssues[0]
	} else {
		return nil
	}
}

func findQaIssue(issues map[string]Issue, targetIssue jira.Issue) (result []Issue) {
	for _, link := range targetIssue.Fields.Issuelinks {
		if (link.Type.Name == "Blocks" && link.OutwardIssue.Key != "" && (link.OutwardIssue.Fields.Issuetype.Name == "QA" || link.OutwardIssue.Fields.Issuetype.Name == "TestCase")) {
			if qaIssue, ok := issues[link.OutwardIssue.Key]; ok {  
				if (qaIssue.Platform == "QA") {
					result = append(result, qaIssue)
				}
			} else {
				qaIssue := Issue {
					Key: link.OutwardIssue.Key,
					Name: link.OutwardIssue.Fields.Summary,
					OutSprint: true,
					Type: link.OutwardIssue.Fields.Issuetype.Name,
					Uri: createUri(link.OutwardIssue.Key),
					IsResolved: link.OutwardIssue.Fields.Status.Category.Key == "done",
				}
				result = append(result, qaIssue)
			}
		}
	}
	return
}

func findDevelopmentIssues(issues map[string]Issue, targetIssue jira.Issue) (result []Issue) {
	for _, link := range targetIssue.Fields.Issuelinks {
		if (link.Type.Name == "Blocks" && link.InwardIssue.Key != "" && link.InwardIssue.Fields.Issuetype.Name != "QA" && link.InwardIssue.Fields.Issuetype.Name != "TestCase" && link.InwardIssue.Fields.Issuetype.Name != "Story") {
			foundIssue, hasIssue := issues[link.InwardIssue.Key]
			var assignee = ""
			var platform = ""
			if (hasIssue) {
				assignee = foundIssue.Assignee
				platform = foundIssue.Platform
			}
			developmentIssue := Issue {
				Assignee: assignee,
				Platform: platform,
				Key: link.InwardIssue.Key,
				Name: link.InwardIssue.Fields.Summary,
				OutSprint: !hasIssue,
				Type: link.InwardIssue.Fields.Issuetype.Name,
				Uri: createUri(link.InwardIssue.Key),
				IsResolved: link.InwardIssue.Fields.Status.Category.Key == "done",
			}
			result = append(result, developmentIssue)
		}
	}
	return
}

func createUri(key string) string {
	return "https://jr.avito.ru/browse/" + key
}

func contains(v interface{}, in interface{}) (ok bool) {
	var i int
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

func hasTestsCassesSubstring(title string) (result bool) {
	lowerTitle := strings.ToLower(title)
	result = false
	result = result || strings.Contains(lowerTitle, "тесткейс")
	result = result || strings.Contains(lowerTitle, "тест-кейс")
	result = result || strings.Contains(lowerTitle, "test casse")
	result = result || strings.Contains(lowerTitle, "test-casse")
	result = result || strings.Contains(lowerTitle, "test case")
	result = result || strings.Contains(lowerTitle, "test-case")
	return
}

func calculateDatesDelta(startDate string, finishDate string) int {
	startTime, err := time.Parse("2006-01-02", startDate)
	if err != nil {
	    return 0
	}
	finishTime, err := time.Parse("2006-01-02", finishDate)
	if err != nil {
	    return 0
	}
	return int(finishTime.Sub(startTime).Hours() / 24) + 1
}

type Issues struct {
	RequestDate time.Time
	Issues []Issue
}

type PlanningInfo struct {
	MaxStoryPoints float64 `json:"maxStoryPoints"`
	Users map[string]User `json:"users"`
}

type User struct {
    Name string `json:"name"`
	PlannedIssues []Issue `json:"plannedIssues"`
	LostIssues []Issue `json:"lostIssues"`
}

type Issue struct {
    Key string `json:"key"`
    Name string `json:"name"`
    Development *Issue `json:"development"`
    QA *Issue `json:"qa"`
    Epic *Issue `json:"epic"`
    TestCases *Issue `json:"testCasses"`
    StoryPoints *float64 `json:"storyPoints"`
    Type string `json:"type"`
    Assignee string `json:"assignee"`
    Platform string `json:"platform"`
    OutSprint bool `json:"outSprint"`
    IsResolved bool `json:"isResolved"`
    IsEasy bool `json:"isEasy"`
    Uri string `json:"uri"`
}