package chore

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	chModel "donetick.com/core/internal/chore/model"
)

func scheduleNextDueDate(chore *chModel.Chore, completedDate time.Time) (*time.Time, error) {
	// if Chore is rolling then the next due date calculated from the completed date, otherwise it's calculated from the due date
	var baseDate time.Time
	var frequencyMetadata chModel.FrequencyMetadata
	err := json.Unmarshal([]byte(*chore.FrequencyMetadata), &frequencyMetadata)
	if err != nil {
		return nil, fmt.Errorf("error unmarshalling frequency metadata")
	}
	if chore.FrequencyType == "once" {
		return nil, nil
	}

	if chore.NextDueDate != nil {
		baseDate = chore.NextDueDate.UTC()
	} else {
		baseDate = completedDate.UTC()
	}

	if chore.FrequencyType == "day_of_the_month" || chore.FrequencyType == "days_of_the_week" || chore.FrequencyType == "interval" {
		t, err := time.Parse(time.RFC3339, frequencyMetadata.Time)
		if err != nil {
			return nil, fmt.Errorf("error parsing time in frequency metadata")
		}
		baseDate = time.Date(baseDate.Year(), baseDate.Month(), baseDate.Day(), t.Hour(), t.Minute(), 0, 0, t.Location())
	}

	if chore.IsRolling {
		baseDate = completedDate.UTC()
	}

	// Calculate next due date using existing logic
	nextDueDate, err := calculateBasicNextDueDate(chore, completedDate, baseDate)
	if err != nil {
		return nil, err
	}
	if nextDueDate == nil {
		return nil, nil
	}

	// Apply period constraints if they exist
	return adjustForPeriod(chore, nextDueDate)
}

// adjustForPeriod adjusts the next due date based on the chore's active period
func adjustForPeriod(chore *chModel.Chore, nextDueDate *time.Time) (*time.Time, error) {
	// If there's no active period, return the calculated date
	if chore.PeriodStart == nil || chore.PeriodEnd == nil {
		return nextDueDate, nil
	}

	// Adjust the year of period dates to match the next due date's year
	currentYear := nextDueDate.Year()
	periodStart := time.Date(currentYear,
		chore.PeriodStart.Month(),
		chore.PeriodStart.Day(),
		chore.PeriodStart.Hour(),
		chore.PeriodStart.Minute(),
		0, 0, time.UTC)
	periodEnd := time.Date(currentYear,
		chore.PeriodEnd.Month(),
		chore.PeriodEnd.Day(),
		chore.PeriodEnd.Hour(),
		chore.PeriodEnd.Minute(),
		0, 0, time.UTC)

	// If next due date is before period start, use period start
	if nextDueDate.Before(periodStart) {
		return &periodStart, nil
	}

	// If next due date is after period end, skip to next year's period start
	if nextDueDate.After(periodEnd) {
		nextYearStart := time.Date(currentYear+1,
			chore.PeriodStart.Month(),
			chore.PeriodStart.Day(),
			chore.PeriodStart.Hour(),
			chore.PeriodStart.Minute(),
			0, 0, time.UTC)
		return &nextYearStart, nil
	}

	// Date falls within period, use it as is
	return nextDueDate, nil
}

// Move all the frequency type specific logic to calculateBasicNextDueDate
func calculateBasicNextDueDate(chore *chModel.Chore, completedDate, baseDate time.Time) (*time.Time, error) {
	var nextDueDate time.Time

	if chore.FrequencyType == "daily" {
		nextDueDate = baseDate.AddDate(0, 0, 1)
	} else if chore.FrequencyType == "weekly" {
		nextDueDate = baseDate.AddDate(0, 0, 7)
	} else if chore.FrequencyType == "monthly" {
		nextDueDate = baseDate.AddDate(0, 1, 0)
	} else if chore.FrequencyType == "yearly" {
		nextDueDate = baseDate.AddDate(1, 0, 0)
	} else if chore.FrequencyType == "adaptive" {
		// TODO: calculate next due date based on the history of the chore
		diff := completedDate.UTC().Sub(chore.NextDueDate.UTC())
		nextDueDate = completedDate.UTC().Add(diff)
	} else if chore.FrequencyType == "interval" {
		var frequencyMetadata chModel.FrequencyMetadata
		err := json.Unmarshal([]byte(*chore.FrequencyMetadata), &frequencyMetadata)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling frequency metadata")
		}

		if *frequencyMetadata.Unit == "hours" {
			nextDueDate = baseDate.UTC().Add(time.Hour * time.Duration(chore.Frequency))
		} else if *frequencyMetadata.Unit == "days" {
			nextDueDate = baseDate.UTC().AddDate(0, 0, chore.Frequency)
		} else if *frequencyMetadata.Unit == "weeks" {
			nextDueDate = baseDate.UTC().AddDate(0, 0, chore.Frequency*7)
		} else if *frequencyMetadata.Unit == "months" {
			nextDueDate = baseDate.UTC().AddDate(0, chore.Frequency, 0)
		} else if *frequencyMetadata.Unit == "years" {
			nextDueDate = baseDate.UTC().AddDate(chore.Frequency, 0, 0)
		} else {
			return nil, fmt.Errorf("invalid frequency unit, cannot calculate next due date")
		}
	} else if chore.FrequencyType == "days_of_the_week" {
		// ... existing days_of_the_week logic ...
		var frequencyMetadata chModel.FrequencyMetadata
		err := json.Unmarshal([]byte(*chore.FrequencyMetadata), &frequencyMetadata)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling frequency metadata")
		}

		for i := 1; i <= 7; i++ {
			nextDueDate = baseDate.AddDate(0, 0, i)
			nextDay := strings.ToLower(nextDueDate.Weekday().String())
			for _, day := range frequencyMetadata.Days {
				if strings.ToLower(*day) == nextDay {
					return &nextDueDate, nil
				}
			}
		}
	} else if chore.FrequencyType == "day_of_the_month" {
		// ... existing day_of_the_month logic ...
		var frequencyMetadata chModel.FrequencyMetadata
		err := json.Unmarshal([]byte(*chore.FrequencyMetadata), &frequencyMetadata)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling frequency metadata")
		}

		for i := 1; i <= 12; i++ {
			nextDueDate = baseDate.AddDate(0, i, 0)
			nextDueDate = time.Date(nextDueDate.Year(), nextDueDate.Month(), chore.Frequency, nextDueDate.Hour(), nextDueDate.Minute(), 0, 0, nextDueDate.Location())
			nextMonth := strings.ToLower(nextDueDate.Month().String())
			for _, month := range frequencyMetadata.Months {
				if *month == nextMonth {
					return &nextDueDate, nil
				}
			}
		}
	} else if chore.FrequencyType == "no_repeat" || chore.FrequencyType == "trigger" {
		return nil, nil
	} else {
		return nil, fmt.Errorf("invalid frequency type, cannot calculate next due date")
	}

	return &nextDueDate, nil
}

func scheduleAdaptiveNextDueDate(chore *chModel.Chore, completedDate time.Time, history []*chModel.ChoreHistory) (*time.Time, error) {

	history = append([]*chModel.ChoreHistory{
		{
			CompletedAt: &completedDate,
		},
	}, history...)

	if len(history) < 2 {
		if chore.NextDueDate != nil {
			diff := completedDate.UTC().Sub(chore.NextDueDate.UTC())
			nextDueDate := completedDate.UTC().Add(diff)
			return &nextDueDate, nil
		}
		return nil, nil
	}

	var totalDelay float64
	var totalWeight float64
	decayFactor := 0.5 // Adjust this value to control the decay rate

	for i := 0; i < len(history)-1; i++ {
		delay := history[i].CompletedAt.UTC().Sub(history[i+1].CompletedAt.UTC()).Seconds()
		weight := math.Pow(decayFactor, float64(i))
		totalDelay += delay * weight
		totalWeight += weight
	}

	averageDelay := totalDelay / totalWeight
	nextDueDate := completedDate.UTC().Add(time.Duration(averageDelay) * time.Second)

	return &nextDueDate, nil
}
func RemoveAssigneeAndReassign(chore *chModel.Chore, userID int) {
	for i, assignee := range chore.Assignees {
		if assignee.UserID == userID {
			chore.Assignees = append(chore.Assignees[:i], chore.Assignees[i+1:]...)
			break
		}
	}
	if len(chore.Assignees) == 0 {
		chore.AssignedTo = chore.CreatedBy
	} else {
		chore.AssignedTo = chore.Assignees[rand.Intn(len(chore.Assignees))].UserID
	}
	chore.UpdatedAt = time.Now()
}
