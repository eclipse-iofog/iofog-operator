# Changelog

## [v3.0.0-alpha2] - 28 July 2021

* Support for Base URLs for ioFog Controllers

## [v3.0.0-alpha1] - 11 March 2021

* Reproduce entire project using Operator Framework v1.3.0
* Upgrade Operator SDK to 0.15.2 to avoid modules import error
* Remove Cluster Role Binding from Port Manager
* Refactor ControlPlane reconciler runtime into more obvious state machine
* Add Status.Conditions to ControlPlane type and reconciler logic

## [v2.0.1] - 2020-10-02

* Remove Router HTTP Port from Load Balancer

## [v2.0.0] - 2020-08-05

### Features

* Remove Kubelet

### Bugs

* Increase LB timeouts
* Increase Controller readiness probe delay
* Change Router liveness probe to readiness
* Service accounts creation failure
* Fix rollout policies for all components

## [v2.0.0-beta3] - 2020-04-23

### Features

* Use Proxy and Router images in Controller env vars
* Increase wait time for Router IP
* Refactor for parallel reconciliation

### Bugs

* Update go-sdk module with WaitForLoadBalancer fix
* Fix CR errors

## [v2.0.0-beta2] - 2020-04-06

### Features

* Add retries to ioFog Controller client
* Add IsSupportedCustomResource
* Add Proxy service to ControlPlaneSpec
* Add CR helper functions to iofog pkg
* Refactor Kog to ControlPlane and make more optional fields in API type
* Add RouterImage to Kog spec

## [v2.0.0-beta] - 2020-03-12

### Features

* Upgrade go-sdk to v2
* Make PVC creation optional
* Add PV for Controller sqlite db

## [v2.0.0-alpha] - 2020-03-10

### Features

* Replace env var with API call to Controller for default Router
* Add port manager env vars
* Add Skupper loadbalancer IP and router ports to Controller env vars
* Remove connectors
* Add PortManagerImage to Kog ControlPlane
* Add env vars for Port Manager
* Add readiness probe to Port Manager
* Deploy Port Manager
* Deploy Skupper Router

### Bugs

* Consolidate usage of iofog client and reorganize controller reconciliation
* Removes all references to Connector
  
[Unreleased]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0-beta3..HEAD
[v2.0.0-beta2]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0-beta2..v2.0.0-beta3
[v2.0.0-beta2]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0-beta..v2.0.0-beta2
[v2.0.0-beta]: https://github.com/eclipse-iofog/iofog-operator/compare/v2.0.0-alpha..v2.0.0-beta
[v2.0.0-alpha]: https://github.com/eclipse-iofog/iofog-operator/tree/v2.0.0-alpha