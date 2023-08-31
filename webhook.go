package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/sirupsen/logrus"
	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

var (
	runtimeScheme = runtime.NewScheme()
	codecs        = serializer.NewCodecFactory(runtimeScheme)
	deserializer  = codecs.UniversalDeserializer()
)

const (
	admissionWebhookAnnotationMutateKey = "deployment-create-by-uniteddeployment"
)

type WebhookServer struct {
	log    logrus.FieldLogger
	server *http.Server
}

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func (ws *WebhookServer) serve(w http.ResponseWriter, r *http.Request) {
	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}
	if len(body) == 0 {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		ws.log.Errorf("Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect `application/json`", http.StatusUnsupportedMediaType)
		return
	}

	var admissionResponse *admissionv1.AdmissionResponse
	ar := admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, &ar); err != nil {
		ws.log.Errorf("Can't decode body: %v", err)
		admissionResponse = &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	} else {
		ws.log.Infof("Request url path: %v", r.URL.Path)
		if r.URL.Path == "/mutate" {
			admissionResponse = ws.mutate(&ar)
		} else if r.URL.Path == "/validate" {
			admissionResponse = ws.validate(&ar)
		}
	}
	admissionReview := admissionv1.AdmissionReview{}
	if admissionResponse != nil {
		admissionReview.Response = admissionResponse
		if ar.Request != nil {
			admissionReview.Response.UID = ar.Request.UID
		}
	}
	resp, err := json.Marshal(admissionReview)
	if err != nil {
		ws.log.Errorf("Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}
	ws.log.Infof("Ready to write reponse ...")
	if _, err := w.Write(resp); err != nil {
		ws.log.Errorf("Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func (ws *WebhookServer) mutate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	var availableAnnotations map[string]string
	req := ar.Request

	// 检查请求操作是否是 CREATE 或 UPDATE
	if req.Operation != admissionv1.Create && req.Operation != admissionv1.Update {
		ws.log.Errorf("Operation:%s, skipping operations that are not create or update operations", req.Operation)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	if req.Kind.Kind != "Deployment" {
		ws.log.Errorf("Skip mutate for %s/%s\n", req.Namespace, req.Name)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	deployment := appsv1.Deployment{}
	if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
		ws.log.Errorf("Could not unmarshal raw object: %v\n", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	temp := false
	for _, owner := range deployment.OwnerReferences {
		if owner.Kind == "UnitedDeployment" {
			temp = true
			break
		}
	}
	if temp {
		availableAnnotations = deployment.Annotations
		annotations := map[string]string{admissionWebhookAnnotationMutateKey: "true"}
		patchBytes, err := createPatch(availableAnnotations, annotations)
		if err != nil {
			return &admissionv1.AdmissionResponse{
				Result: &metav1.Status{
					Message: err.Error(),
				},
			}
		}
		ws.log.Infof("AdmissionResponse: patch=%v\n", string(patchBytes))
		return &admissionv1.AdmissionResponse{
			Allowed: true,
			Patch:   patchBytes,
			PatchType: func() *admissionv1.PatchType {
				pt := admissionv1.PatchTypeJSONPatch
				return &pt
			}(),
		}
	}
	ws.log.Errorf("Skip mutate for %s/%s\n", req.Namespace, req.Name)
	return &admissionv1.AdmissionResponse{
		Allowed: true,
	}
}

func (ws *WebhookServer) validate(ar *admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
	var availableAnnotations map[string]string

	req := ar.Request
	if req.Kind.Kind != "Deployment" {
		ws.log.Errorf("Skip mutate for %s/%s\n", req.Namespace, req.Name)
		return &admissionv1.AdmissionResponse{
			Allowed: true,
		}
	}

	deployment := appsv1.Deployment{}
	if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
		ws.log.Errorf("Could not unmarshal raw object: %v\n", err)
		return &admissionv1.AdmissionResponse{
			Result: &metav1.Status{
				Message: err.Error(),
			},
		}
	}

	temp := false
	for _, owner := range deployment.OwnerReferences {
		if owner.Kind == "UnitedDeployment" {
			temp = true
			break
		}
	}

	allowed := true
	var result *metav1.Status

	if temp {
		availableAnnotations = deployment.Annotations
		if _, ok := availableAnnotations[admissionWebhookAnnotationMutateKey]; !ok {
			allowed = false
			result = &metav1.Status{
				Reason: "required annotation are not set",
			}
		}
	}
	return &admissionv1.AdmissionResponse{
		Allowed: allowed,
		Result:  result,
	}
}

func createPatch(availableAnnotations map[string]string, annotations map[string]string) ([]byte, error) {
	var patch []patchOperation
	patch = append(patch, updateAnnotation(availableAnnotations, annotations)...)
	return json.Marshal(patch)
}

func updateAnnotation(target map[string]string, added map[string]string) (patch []patchOperation) {
	for key, value := range added {
		if target == nil || target[key] == "" {
			target = map[string]string{}
			patch = append(patch, patchOperation{
				Op:   "add",
				Path: "/metadata/annotations",
				Value: map[string]string{
					key: value,
				},
			})
		} else {
			patch = append(patch, patchOperation{
				Op:    "replace",
				Path:  "/metadata/annotations/" + key,
				Value: value,
			})
		}
	}
	return patch
}
