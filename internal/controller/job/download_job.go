package job

import "github.com/samber/mo"

type DownloadJob struct {
}

func NewDownloadJob() DownloadJob {
	return DownloadJob{}
}

func (job *DownloadJob) GetJobID() int {
	return 0
}

func (job *DownloadJob) GetType() AsyncJobType {
	return 0
}
func (job *DownloadJob) GetState() AsyncJobState {
	return 0
}

func (job *DownloadJob) GetCompletionStatus() mo.Option[AsyncJobStatus] {
	return mo.None[AsyncJobStatus]()
}

func (job *DownloadJob) GetCompletionComment() mo.Option[string] {
	return mo.None[string]()
}
func (job *DownloadJob) Run() {

}
