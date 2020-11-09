# A workshop to use kpt in skaffold 

- [A workshop to use kpt in skaffold ](#a-workshop-to-use-kpt-in-skaffold-)
- [](#)
- [Introduction](#introduction)
   - [What is kpt?](#what-is-kpt)
   - [What kpt can help you in skaffold](#what-kpt-can-help-you-in-skaffold)
   - [What you'll learn](#what-you'll-learn)
- [Prerequisites](#prerequisites)
   - [Install via gcloud SDK](#install-via-gcloud-sdk)
   - [Other Installation options](#other-installation-options)
   - [Verify the installation](#verify-the-installation)
   - [Install Kind](#install-kind)
   - [Create a Kind cluster ](#create-a-kind-cluster-)
- [Getting started](#getting-started)
   - [Download the sample application ](#download-the-sample-application-)
   - [Configure the skaffold.yaml file](#configure-the-skaffoldyaml-file)
   - [Run skaffold](#run-skaffold)
- [Validate the config](#validate-the-config)
      - [Verify the validation "happy path" ](#verify-the-validation-"happy-path"-)
      - [Verify the validation "sad path" ](#verify-the-validation-"sad-path"-)
- [Validate through a pipeline](#validate-through-a-pipeline)
   - [Run as a pipeline](#run-as-a-pipeline)
      - [Verify the validation pipeline "sad path" ](#verify-the-validation-pipeline-"sad-path"-)
- [Deploy](#deploy)
   - ["prune"](#"prune")
   - [Compare the differences](#compare-the-differences)
      - [Kubectl prune](#kubectl-prune)
      - [Kpt prune](#kpt-prune)
- [The GitOps CICD workflow](#the-gitops-cicd-workflow)
      - [TBD: Vic's insights about the branding and introducing MAD. ](#tbd-vic's-insights-about-the-branding-and-introducing-mad-)
- [Cleanup](#cleanup)
   - [Delete the Kind cluster](#delete-the-kind-cluster)
- [Congratulations](#congratulations)

---


![image](https://sites.google.com/a/google.com/d2html-img/users/yuwenma/skaffoldcodela--xnfmoo7pj39.png)

_Last Updated: 2020-10-30_

# Introduction

## What is kpt?

Kpt is [an OSS tool](https://github.com/GoogleContainerTools/kpt) for Kubernetes packaging, which uses a standard format to bundle, publish, customize, update, and apply configuration manifests.

## What kpt can help you in skaffold

-  You will get an hand-on off-the-shelf experience about the **GitOps** CI/CD workflow in skaffold.
-  You can validate each of your config changes **declaratively**.
-  You **won't **encounter** version conflict** if the config hydration (a.k.a kustomize) mismatch with the deployment tool (e.g. kubectl). 
-  You can prune your resources accurately with [a three-way merge strategy](https://kubectl.docs.kubernetes.io/pages/app_management/field_merge_semantics.html). 

## What you'll learn

-  How to add a validation to your config changes. 
-  How to define validation rules in the form of a declarative pipeline.
-  How to use the validation in kustomized resources. 
-  How to reconcile your configuration changes with the live state

# Prerequisites

If you are new to skaffold, you can check out the [skaffold tutorials](https://skaffold.dev/docs/tutorials/) to get a basic idea. Or just follow this codelab, we will explain what happens in each step.

## Install via gcloud SDK

This installs skaffold,  kpt and kustomize.
```bash
gcloud components install pkg 
gcloud components install skaffold
```

## Other Installation options

See more download options in the official download page. 

-  Download `kpt`  from [here](https://googlecontainertools.github.io/kpt/installation/)
-  Download `skaffold` from [here](https://skaffold.dev/docs/install/)
-  Download `kustomize` from [here](https://kubernetes-sigs.github.io/kustomize/installation/)  (**optional**. Only install when you have kustomization.yaml files in your repository)

## Verify the installation

```bash
skaffold version
kpt version 
kustomize version
```

`skaffold` version is required to be >= v1.14.0  
`kpt` version is required to be >= 0.34.0  
`kustomize` version is required to be >= v3.4.3 

## Install Kind

This codelab uses [kind](https://kind.sigs.k8s.io/) to bring up a local kubernetes cluster. We configure  skaffold to use  kpt to deploy the example application to this cluster.  
   
Download `kind` from [here](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)

<table>
<thead>
<tr>
<th><strong>Note:</strong> To use kind, you must have docker installed. See docker installation <a href="https://docs.docker.com/get-docker/">here</a></th>
</tr>
</thead>
<tbody>
</tbody>
</table>

## Create a Kind cluster 

```bash
kind create cluster --name kpt-cl
```

# Getting started

Duration: 00:10:00

## Download the sample application 

We use `kpt pkg` to get the application example and store it to your local directory `guestbook-cl `

```bash
kpt pkg get https://github.com/yuwenma/sample-app guestbook-cl && cd guestbook-cl
```

<table>
<thead>
<tr>
<th>Extended Reading<br>
<code>kpt pkg</code> downloads the resource from a remote repository, a branch or a subdirectory. <code>kpt pkg</code> does not contain the git version control history but only the specific git reference, so you can compose your own "package" from multiple repositories.  Read more about the kpt package from <a href="https://googlecontainertools.github.io/kpt/reference/pkg/">here</a>.</th>
</tr>
</thead>
<tbody>
</tbody>
</table>

## Configure the skaffold.yaml file

All skaffold configurations are stored in file `./skaffold.yaml`. The  `guestbook-cl` has configured the `skaffold.yaml` for you. Please take a look at the skaffold.yaml file below

skaffold.yaml
```yaml
apiVersion: skaffold/v2beta8
kind: Config
metadata:
  name: kpt-cl
build:
  artifacts:
  - image: "frontend"
    context: php-redis
deploy:
  kpt:
    dir: config
```

The file contains two stages: `build` and `deploy`. "build" defines the methods to **build** and **upload** the application images (by default, it uses docker); "deploy" defines the methods to **manage** the app configuration and **deploy** the application to the bundled cluster.

In our example, the configurations will build an image "frontend" and use `kpt` to hydrate[1] and deploy the applications to the kind cluster. The "frontend" source code is stored in `./php-redis` and its configurations are stored in `./config`.

<table>
<thead>
<tr>
<th>Glossary<br>
[1] <code>Hydrate </code>means rendering a kustomize directory or a kpt package to a flatten configuration, each of whose resources contains the full set of the object information.</th>
</tr>
</thead>
<tbody>
</tbody>
</table>

## Run skaffold

```bash
skaffold dev
```

`skaffold dev` is the essential skaffold command. It builds the application and then deploys the applications to the bundled cluster. Once the deployment is complete, you can exit with `Ctrl+C`.

<table>
<thead>
<tr>
<th><strong>Note:</strong> Please ignore the domain name warning. Since we are using the kind cluster, we only bring up a frontend application. The complete example "guestbook" can be found in the <a href="https://github.com/kubernetes/examples">kubernetes example </a>repository. </th>
</tr>
</thead>
<tbody>
</tbody>
</table>

# Validate the config

Duration: 00:20:00

Validating the configurations helps both the app development and devOps to be efficient in a fragile environment. 

This step uses a `kubeval` example to show how `kpt` functions[2] can validate the app configuration and makes the validation itself **as a declarative config.**

<table>
<thead>
<tr>
<th>Glossary<br>
[2] <code>kpt function </code>is a kubernetes resource with a <code>config.kubernetes.io/function</code> annotation. Read more <a href="https://googlecontainertools.github.io/kpt/concepts/functions/">here</a>  </th>
</tr>
</thead>
<tbody>
</tbody>
</table>

To download the resource, please run:

```bash
kpt pkg get https://github.com/yuwenma/kpt-funcs-example.git/validations/kubeval validation-kubeval
```

<table>
<thead>
<tr>
<th><strong>Note:</strong> this is not a git URL, so <code>git clone </code>won't work. The <code>kpt</code> package URL should be in the form of <code>REPO_URI[.git]/PKG_PATH[@VERSION]</code></th>
</tr>
</thead>
<tbody>
</tbody>
</table>

Now let's update the skaffold.yaml to use the new validator. Add the following code in the `.deploy.kpt` section in your `skaffold.yaml`   
**[skaffold.yaml](https://github.com/yuwenma/sample-app/blob/kubeval/skaffold.yaml#L12)**

```yaml
    fn:
      fnPath: validation-kubeval
      network: true
```

-  `.deploy.kpt.fn.fnPath` refers to the kpt function directory we just downloaded. 
-  `.deploy.kpt.fn.network `enables the kpt access to the network. This is required to run the function in a docker container.

### Verify the validation "happy path" 

`skaffold dev` has `kpt `embedded so as the resource validation happens **before** deploying to the kind cluster.

```bash
skaffold dev
```

### Verify the validation "sad path" 

Do not exit `skaffold dev` and let's fail the validation! 

<table>
<thead>
<tr>
<th>Tips: <code>skaffold dev</code> can automatically detect file changes and kick off a re-deploy. So you don't need to rerun the command if the file changes.</th>
</tr>
</thead>
<tbody>
</tbody>
</table>

Delete `spec.template.spec.containers.image`  in `config/frontend/deployment.yaml` **[line19**](https://github.com/yuwenma/sample-app/blob/master/config/frontend/deployment.yaml#L19). This "image" is a required field. Now the `Deployment` is no longer a valid schema. We expect kubeval can catch this error.     

For Linux
```bash
sed -i '19d' ./config/frontend/deployment.yaml
```

For MacOS
```bash
sed -i.tmp 19d ./config/frontend/deployment.yaml && rm ./config/frontend/deployment.yaml.tmp
```

Check the `skaffold dev` output. You should see the following warning from the terminal.

![image](https://sites.google.com/a/google.com/d2html-img/users/yuwenma/skaffoldcodela--hb38cmy4b78.png)

Add back the removed line and make it work again.  

For Linux
```bash
sed -i '19 a \        image: "frontend"' ./config/frontend/deployment.yaml
```

For MacOS
```bash
sed -i "" -e $'18 a\\\n\        image: "frontend"' ./config/frontend/deployment.yaml 
```

<table>
<thead>
<tr>
<th>Tips: You can find all the <code>kpt</code> validation functions from this  <a href="https://googlecontainertools.github.io/kpt/guides/consumer/function/catalog/validators/">catalog</a>. Or write your own versions. See <a href="https://googlecontainertools.github.io/kpt/guides/consumer/function/">instructions</a>.  </th>
</tr>
</thead>
<tbody>
</tbody>
</table>

# Validate through a pipeline

Instead of using a single validation function, we are more interested in applying the configurations through a series of validation functions, each of which checks a specific rule.

To do so, you can put a list of `kpt` functions into a single file and kpt will apply the resources through these functions based on the function orders in the single file. Like the graph shows.

![image](https://sites.google.com/a/google.com/d2html-img/users/yuwenma/skaffoldcodela--yvbbp5pxqzo.png)

## Run as a pipeline

Let's go through an example.   
 

-  Download the resource. This resource contains two validation functions. A "kubeval" validator to check the_ yaml schema_, and an "example-validator-kubeval" validator to check if all containers have the _CPU and memory reservation_ set.   
```bash
kpt pkg get https://github.com/yuwenma/kpt-funcs-example.git/validations/pipeline validations
```
-  Update the skaffold.yaml  to point to the new function directory. See the full **[skaffold.yaml](https://github.com/yuwenma/sample-app/blob/pipeline/skaffold.yaml#L13)**
```yaml
    fn:
      fnPath: validations
      network: true
```

-  Check the result. 
```bash
skaffold dev
```

### Verify the validation pipeline "sad path" 

Let's  break the  "example-validator-kubeval" validator. In config/frontend/deployment.yaml, remove the container's `cpu` (line 22)  
For Linux
```bash
sed -i '22d' ./config/frontend/deployment.yaml
```

For MacOS
```bash
sed -i.tmp 22d ./config/frontend/deployment.yaml && rm ./config/frontend/deployment.yaml.tmp
```

Check the `skaffold dev` output. You should see the following warning from the terminal.

# Deploy

`Skaffold `improves its deployment by using` kpt `to reconcile the resources in their live states. One key improvement is "pruning" the resource more accurately. 

## "prune"

You may already know that `kubectl apply --prune -f DIR` can remove resources if they are not shown in the DIR. Or you may use `kubectl delete` to remove resources specifically. However,  both approaches could remove the wrong resource and thus put your skaffold application in a dangerous situation.   

kpt uses a [three-way merge strategy](https://kubectl.docs.kubernetes.io/pages/app_management/field_merge_semantics.html) to detect resources more accurately and make the changes more wisely. 

<table>
<thead>
<tr>
<th>Extended reading<br>
<a href="https://github.com/kubernetes/enhancements/pull/810/files">This KEP</a> gives a full context about the <code>kubectl</code> "prune"  issue. Thanks <a href="https://github.com/Liujingfang1">Liujingfang1</a> for the contribution.<br>
<br>
This is <a href="https://github.com/kubernetes/kubernetes/issues/66430">a real user problem</a> that hasn't been fixed for 2 years.</th>
</tr>
</thead>
<tbody>
</tbody>
</table>

## Compare the differences

### Kubectl prune

Let's use kubectl to prune the resources. Here's the skaffold.yaml you can use

Create a new kind cluster

```bash
kind create cluster --name kubectl-prune-cl 
```

Use `kubectl` in skaffold.yaml and enables the `--prune`.The[ skaffold.yaml](https://github.com/yuwenma/sample-app/blob/kubectl-prune/skaffold.yaml#L13) should look like this 
```yaml
apiVersion: skaffold/v2beta8
kind: Config
metadata:
  name: kpt-cl
build:
  artifacts:
  - image: "frontend"
    context: php-redis
deploy:
  kubectl:
    flags:
      apply:
      - "--prune=true"
      - "--all=true"
      - "--namespace=default"
    manifests:
    - config/frontend/*.yaml
```

Run `skaffold dev`
```bash
skaffold dev 
```

Open a new terminal to move the deployment.yaml out of the `deploy.kubectl.manifests` path

```bash
mv config/frontend/deployment.yaml .
```

Check the result. skaffold can not rely on kubectl to correctly prune the deployment.  

```yaml
kubectl get deployment
NAME       READY   UP-TO-DATE   AVAILABLE   AGE
frontend   1/1     1            1           6m17s
```

### Kpt prune

Now let's switch to use kpt. 

Add back the deployment.yaml

```bash
mv deployment.yaml config/frontend/
```

Switch back the cluster 

```bash
kubectl cluster-info --context kind-kpt-cl
```

Update skaffold.yaml
```yaml
apiVersion: skaffold/v2beta8
kind: Config
metadata:
  name: kpt-cl
build:
  artifacts:
  - image: "frontend"
    context: php-redis
deploy:
  kpt:
    dir: config
```

Run `skaffold dev`

```bash
skaffold dev 
```

Remove the deployment.yaml from the kustomization.yaml  
For Linux
```bash
sed -i '2d' ./config/kustomization.yaml
```

For MacOS
```bash
sed -i.tmp 2d ./config/kustomization.yaml && rm ./config/kustomization.yaml.tmp
```

Check the`skaffold dev` output

![image](https://sites.google.com/a/google.com/d2html-img/users/yuwenma/skaffoldcodela--txpndx5ov2r.png)

Check in the cluster

```bash
kubectl get deployment
```

# The GitOps CICD workflow

### TBD: Vic's insights about the branding and introducing MAD. 

# Cleanup

## Delete the Kind cluster

```bash
kind delete cluster --name kpt-cl
kind delete cluster --name kubectl-prune-cl
```
---

# Congratulations

Duration: 0:00

Congratulations, you've known how to use kpt in skaffold! You can explore other kpt features from the skaffold.yaml[ reference doc](https://skaffold.dev/docs/references/yaml/#deploy-kpt). 

You can also try out other kpt features like `kpt pkg` and `kpt cfg` from [the user guide](https://googlecontainertools.github.io/kpt/reference/). They will be supported in the skaffold soon. Stay tuned!  