package calendar

import (
	"time"

	dbus "github.com/godbus/dbus"
	libdate "github.com/rickb777/date"
	"pkg.deepin.io/lib/dbusutil"
)

const (
	dbusServiceName = "com.deepin.daemon.Calendar"
	dbusPath        = "/com/deepin/daemon/Calendar/Scheduler"
	dbusInterface   = "com.deepin.daemon.Calendar.Scheduler"
)

type queryJobsParams struct {
	Key   string
	Start time.Time
	End   time.Time
}

func (s *Scheduler) QueryJobs(params string) (jobsJSON string, busErr *dbus.Error) {
	var ps queryJobsParams
	err := fromJson(params, &ps)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	jobs, err := s.queryJobs(ps.Key, ps.Start, ps.End)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	jobsJSON, err = toJson(jobs)
	return jobsJSON, dbusutil.ToError(err)
}

func (s *Scheduler) QueryJobsWithLimit(params string, maxNum int32) (jobsJSON string, busErr *dbus.Error) {
	var ps queryJobsParams
	err := fromJson(params, &ps)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	jobs, err := s.queryJobsWithLimit(ps.Key, ps.Start, ps.End, maxNum)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	jobsJSON, err = toJson(jobs)
	return jobsJSON, dbusutil.ToError(err)
}

func (s *Scheduler) QueryJobsWithRule(params string, rule string) (jobsJSON string, busErr *dbus.Error) {
	var ps queryJobsParams
	err := fromJson(params, &ps)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	jobs, err := s.queryJobsWithRule(ps.Key, ps.Start, ps.End, rule)
	if err != nil {
		return "", dbusutil.ToError(err)
	}

	jobsJSON, err = toJson(jobs)
	return jobsJSON, dbusutil.ToError(err)
}

func (s *Scheduler) GetJobs(startYear, startMonth, startDay, endYear, endMonth, endDay int32) (jobsJSON string, busErr *dbus.Error) {
	startDate := libdate.New(int(startYear), time.Month(startMonth), int(startDay))
	endDate := libdate.New(int(endYear), time.Month(endMonth), int(endDay))
	jobs, err := s.getJobs(startDate, endDate)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	jobsJSON, err = toJson(jobs)
	return jobsJSON, dbusutil.ToError(err)
}

func (s *Scheduler) GetJobsWithLimit(startYear, startMonth, startDay, endYear,
	endMonth, endDay int32, maxNum int32) (jobsJSON string, busErr *dbus.Error) {

	startDate := libdate.New(int(startYear), time.Month(startMonth), int(startDay))
	endDate := libdate.New(int(endYear), time.Month(endMonth), int(endDay))
	jobs, err := s.getJobsWithLimit(startDate, endDate, maxNum)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	jobsJSON, err = toJson(jobs)
	return jobsJSON, dbusutil.ToError(err)
}

func (s *Scheduler) getJobsWithLimit(startDate, endDate libdate.Date, maxNum int32) ([]dateJobsWrap, error) {
	var allJobs []*Job
	err := s.db.Find(&allJobs).Error
	if err != nil {
		return nil, err
	}
	t0 := time.Now()
	wraps := getJobsBetween(startDate, endDate, allJobs, false, "", false)
	result := takeFirstNJobs(wraps, maxNum)
	logger.Debug("cost time:", time.Since(t0))
	return result, nil
}

func (s *Scheduler) GetJobsWithRule(startYear, startMonth, startDay,
	endYear, endMonth, endDay int32, rule string) (jobsJSON string, busErr *dbus.Error) {

	startDate := libdate.New(int(startYear), time.Month(startMonth), int(startDay))
	endDate := libdate.New(int(endYear), time.Month(endMonth), int(endDay))
	jobs, err := s.getJobsWithRule(startDate, endDate, rule)
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	jobsJSON, err = toJson(jobs)
	return jobsJSON, dbusutil.ToError(err)
}

func (s *Scheduler) getJobsWithRule(startDate, endDate libdate.Date, rule string) ([]dateJobsWrap, error) {
	allJobs, err := getJobsWithRule(s.db, rule)
	if err != nil {
		return nil, err
	}
	var result []dateJobsWrap
	t0 := time.Now()
	wraps := getJobsBetween(startDate, endDate, allJobs, true, "", false)
	for _, item := range wraps {
		if item.jobs != nil {
			result = append(result, item)
		}
	}
	logger.Debug("cost time:", time.Since(t0))
	return result, nil
}

func (s *Scheduler) GetJob(id int64) (jobJSON string, busErr *dbus.Error) {
	job, err := s.getJob(uint(id))
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	jobJSON, err = toJson(job)
	return jobJSON, dbusutil.ToError(err)
}

func (s *Scheduler) DeleteJob(id int64) *dbus.Error {
	err := s.deleteJob(uint(id))
	if err == nil {
		s.notifyJobsChange(uint(id))
	}
	return dbusutil.ToError(err)
}

func (s *Scheduler) UpdateJob(jobJSON string) *dbus.Error {
	var jj JobJSON
	err := fromJson(jobJSON, &jj)
	if err != nil {
		return dbusutil.ToError(err)
	}

	job, err := jj.toJob()
	if err != nil {
		return dbusutil.ToError(err)
	}
	err = s.updateJob(job)
	if err == nil {
		s.notifyJobsChange(job.ID)
	}
	return dbusutil.ToError(err)
}

func (s *Scheduler) CreateJob(jobJSON string) (id int64, busErr *dbus.Error) {
	var jj JobJSON
	err := fromJson(jobJSON, &jj)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}

	job, err := jj.toJob()
	if err != nil {
		return 0, dbusutil.ToError(err)
	}
	err = s.createJob(job)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}
	s.notifyJobsChange(job.ID)
	return int64(job.ID), nil
}

func (s *Scheduler) GetTypes() (typesJSON string, busErr *dbus.Error) {
	types, err := s.getTypes()
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	typesJSON, err = toJson(types)
	return typesJSON, dbusutil.ToError(err)
}

func (s *Scheduler) GetType(id int64) (typeJSON string, busErr *dbus.Error) {
	t, err := s.getType(uint(id))
	if err != nil {
		return "", dbusutil.ToError(err)
	}
	typeJSON, err = toJson(t)
	return typeJSON, dbusutil.ToError(err)
}

func (s *Scheduler) DeleteType(id int64) *dbus.Error {
	err := s.deleteType(uint(id))
	return dbusutil.ToError(err)
}

func (s *Scheduler) CreateType(typeJSON string) (id int64, busErr *dbus.Error) {
	var jobType JobTypeJSON
	err := fromJson(typeJSON, &jobType)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}

	jt := jobType.toJobType()
	err = s.createType(jt)
	if err != nil {
		return 0, dbusutil.ToError(err)
	}
	return int64(jt.ID), nil
}

func (s *Scheduler) UpdateType(typeJSON string) *dbus.Error {
	var jobType JobTypeJSON
	err := fromJson(typeJSON, &jobType)
	if err != nil {
		return dbusutil.ToError(err)
	}

	jt := jobType.toJobType()
	err = s.updateType(jt)
	return dbusutil.ToError(err)
}
