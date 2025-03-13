package etcd_func

import (
	"net/http"

	"github.com/holdno/gopherCron/app"
	"github.com/holdno/gopherCron/cmd/service/response"
	"github.com/holdno/gopherCron/common"
	"github.com/holdno/gopherCron/errors"
	"github.com/holdno/gopherCron/utils"

	"github.com/gin-gonic/gin"
)

type KillTaskRequest struct {
	ProjectID int64  `json:"project_id" form:"project_id" binding:"required"`
	TaskID    string `json:"task_id" form:"task_id" binding:"required"`
}

// KillTask kill task from etcd
// post project & task name
// 一般强行结束任务就是想要停止任务 所以这里对任务的状态进行变更
func KillTask(c *gin.Context) {
	var (
		err  error
		req  KillTaskRequest
		task *common.TaskInfo
		uid  = utils.GetUserID(c)
		srv  = app.GetApp(c)
		// lk   *etcd.Locker
	)

	if err = utils.BindArgsWithGin(c, &req); err != nil {
		response.APIError(c, err)
		return
	}

	// 这里主要是防止用户强杀不属于自己项目的业务
	if err = srv.CheckPermissions(req.ProjectID, uid, app.PermissionView); err != nil {
		response.APIError(c, err)
		return
	}

	if err = srv.KillTask(req.ProjectID, req.TaskID); err != nil {
		response.APIError(c, err)
		return
	}

	// 强杀任务后暂停任务
	if task, err = srv.GetTask(req.ProjectID, req.TaskID); err != nil {
		cerr := errors.NewError(http.StatusInternalServerError, "获取任务信息失败")
		cerr.Log = err.Error()
		response.APIError(c, cerr)
		return
	}
	if task == nil {
		response.APIError(c, errors.NewError(http.StatusNoContent, "任务不存在"))
		return
	}
	// lk = srv.GetTaskLocker(task)
	// // 锁释放则证明任务结束
	// ctx, cancel := context.WithTimeout(c, time.Second*10)
	// defer cancel()
	// for {
	// 	select {
	// 	case <-ctx.Done():
	// 		response.APIError(c, ctx.Err())
	// 		return
	// 	default:
	// 		if err = lk.TryLock(); err != nil {
	// 			time.Sleep(time.Second)
	// 			continue
	// 		}
	// 	}
	// 	break
	// }
	// defer lk.Unlock()

	task.Status = 0
	if _, err = srv.SaveTask(task); err != nil {
		cerr := errors.NewError(http.StatusInternalServerError, "强行关闭任务成功，暂停任务失败")
		cerr.Log = err.Error()
		cerr.MsgEn = "task killed, not stop"
		response.APIError(c, err)
		return
	}
	// if err = srv.SetTaskNotRunning(*task); err != nil {
	// 	goto ChangeStatusError
	// }
	response.APISuccess(c, nil)
}
