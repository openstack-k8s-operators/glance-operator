/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package glance

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"errors"
)

// DeleteJob func
func DeleteJob(job *batchv1.Job, client client.Client, log logr.Logger) (bool, error) {

	// Check if this Job already exists
	foundJob := &batchv1.Job{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob)
	if err == nil {
		log.Info("Deleting Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
		err = client.Delete(context.TODO(), foundJob)
		if err != nil {
			return false, err
		}
		return true, err
	}
	return false, nil
}

// EnsureJob func
func EnsureJob(job *batchv1.Job, client client.Client, log logr.Logger) (bool, error) {
	// Check if this Job already exists
	foundJob := &batchv1.Job{}
	err := client.Get(context.TODO(), types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob)
	if err != nil && k8s_errors.IsNotFound(err) {
		log.Info("Creating a new Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
		err = client.Create(context.TODO(), job)
		if err != nil {
			return false, err
		}
		return true, err
	} else if err != nil {
		log.Info("EnsureJob err")
		return true, err
	} else if foundJob != nil {
		log.Info("EnsureJob foundJob")
		if foundJob.Status.Active > 0 {
			log.Info("Job Status Active... requeuing")
			return true, err
		} else if foundJob.Status.Failed > 0 {
			log.Info("Job Status Failed")
			return true, k8s_errors.NewInternalError(errors.New("Job Failed. Check job logs"))
		} else if foundJob.Status.Succeeded > 0 {
			log.Info("Job Status Successful")
		} else {
			log.Info("Job Status incomplete... requeuing")
			return true, err
		}
	}
	return false, nil

}
