var Chart = {

  draw : function (sprint) {
    var groupedIssues = calculateCategories(sprint.issues)
    var categories = Object.keys(groupedIssues)
    var categoriesIssues = categories.map(category => groupedIssues[category])
    var chart = new Highcharts.Chart({
      chart: {
          renderTo: 'chart',
          type: 'bar',
          spacingBottom: 30,
          spacingTop: 65,
          spacingLeft: 10,
          spacingRight: 10
      },
      title: {
        text: '',
        x: -20 //center
      },
      subtitle: {
        text: '',
        x: -20
      },
      plotOptions: {
        series: {
          stacking: 'normal',
          animation: false
        }
      },
      xAxis: {
        categories: categories
      },
      yAxis: {
        title: {
            text: 'Story Points',
        }
      },
      legend: {
        layout: 'vertical',
        align: 'right',
        verticalAlign: 'middle',
        borderWidth: 0,
        reversed: true
      },
      series: [{
        color: '#fc6267',
        name: 'Backend',
        borderWidth: 0,
        data: calculateCategoryStories(categoriesIssues, "Backend")
      }, {
        color: '#a06ef4',
        name: 'Frontend',
        borderWidth: 0,
        data: calculateCategoryStories(categoriesIssues, "Frontend")
      }, {
        color: '#98cd38',
        name: 'Android',
        borderWidth: 0,
        data: calculateCategoryStories(categoriesIssues, "Android")
      }, {
        color: '#95afc0',
        name: 'iOS',
        borderWidth: 0,
        data: calculateCategoryStories(categoriesIssues, "iOS")
      }, {
        color: '#1dacfc',
        name: 'QA',
        borderWidth: 0,
        data: calculateCategoryStories(categoriesIssues, "QA")
      }]
    });

    function calculateCategories(issues) {
      return issues.reduce(function (result, issue) {
          issue.parents.forEach(function(parent) {
            result[parent] = result[parent] || [];
            result[parent].push(issue);
          })
          return result;
      }, Object.create(null));
    }

    function calculateCategoryStories(categoriesIssues, category) {
      var currentTime = new Date().getTime()
      return categoriesIssues.map(function(issues) {
        var filteredIssues = issues
          .filter(issue => issue.platforms.some(platform => platform == category))
        var storyPoints = filteredIssues
          .filter(function (issue) { 
            var closeTime
            if (issue.closeDate != null) {
              closeTime = issue.closeDate.getTime()
            } else {
              closeTime = currentTime + 1
            }
            return closeTime > currentTime
          })
          .map(issue => issue.storyPoints + issue.childrenStories)
          .reduce (function(result, storyPoints){
            return result + storyPoints
          }, 0)

        var borderWidth = 0
        if (filteredIssues.some(issue => issue.isProgress)) {
          borderWidth = 2
        }

        return { 
          borderWidth: borderWidth,
          y: storyPoints
        }

      })
    }

  }

};