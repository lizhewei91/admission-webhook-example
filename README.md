# Admission-webhook-example
该项目是开发 admission-webhook 的一个简单demo，功能主要分两部分：mutate 和 validate
* mutate 是给 `deployment.OwnerReferences.Kind =="UnitedDeployment"`的deployment资源增加一个annotation, `"deployment-create-by-uniteddeployment"="true"`
* validata 则是验证 `deployment.OwnerReferences.Kind =="UnitedDeployment"`的deployment，是否存在`"deployment-create-by-uniteddeployment"="true"`的annotation，
存在则放行，不存在拒绝该请求。

## 构建
```shell
make
```