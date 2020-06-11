# Using ASICs

_WARNING: you are on the master branch, please refer to the docs on the branch that matches your `cortex version`_

## Cortex

To use ASICs (Inferentia):

1. You may need to [file an AWS support ticket](https://console.aws.amazon.com/support/cases#/create?issueType=service-limit-increase&limitType=ec2-instances) to increase the limit for your desired instance type.
1. Set instance type to an AWS Inferentia instance (e.g. `inf1.xlarge`) when installing Cortex.
1. Set the `asic` field in the `compute` configuration for your API. One unit of ASIC corresponds to one virtual ASIC. Fractional requests are not allowed.

## Neuron

Cortex supports one type of ASICs: [AWS Inferentia ASICs](https://aws.amazon.com/machine-learning/inferentia/).

These ASICs come in different sizes depending on the instance type:

* `inf1.xlarge`/`inf1.2xlarge` - each has 1 Inferentia chip.
* `inf1.6xlarge` - has 4 Inferentia chips.
* `inf1.24xlarge` - has 16 Inferentia chips.

Each Inferentia chip comes with 4 Neuron Cores and 8GB of cache memory. To better understand how ASICs (Inferentia) work, read these [technical notes](https://github.com/aws/aws-neuron-sdk/blob/master/docs/technotes/README.md) and this [FAQ](https://github.com/aws/aws-neuron-sdk/blob/master/FAQ.md).

### NeuronCore Groups

An NCG ([*NeuronCore Group*](https://github.com/aws/aws-neuron-sdk/blob/master/docs/tensorflow-neuron/tutorial-NeuronCore-Group.md)) is a set of Neuron Cores that is used to load and run a compiled model. At any point in time, only one model will be running in an NCG. Models can also be shared within an NCG, but for that to happen, the device driver is going to have to dynamically context switch between each model - therefore the Cortex team has decided to only allow one model per NCG to improve performance. The compiled output models are saved in the same format as the source's.

NCGs exist for the sole purpose of aggregating Neuron Cores to improve hardware performance. It is advised to set the NCGs' size to that of the compiled model's within your API. The NCGs' size is determined indirectly using the available number of ASIC chips to the API and the number of workers per replica.

Determining the maximum value for [`workers_per_replica`](autoscaling.md#replica-parallelism) for `inf1` instances can be calculated using following formula:

```text
{2^i; 1 <= i <= 4 * compute:asic / model_no_cores, i is int}
```

`compute:asic` represents the number of ASICs used per API replica and `model_no_cores` is the number of cores for which the model has been compiled. For example, a model that has been compiled to use 1 neuron core and an API that uses 1 ASIC will allow [`workers_per_replica`](autoscaling.md#replica-parallelism) to be set to 1, 2 or 4 - in this case, for 1 worker, the model will be loaded within an NCG (*NeuronCore Group*) of size 4, for 2 it will be an NCG of size 2 and for 4 it's an NCG of size 1. For Tensorflow Predictors that use ASICs, the models will always be placed in different NCGs to avoid context-switching. To better understand what NCGs are, check out the [ASIC instructions](asics.md).

The NCG's compute and memory resources scale along with the NCG's size - an 8-sized NCG will have at its disposal 2x8GB of cache memory and 8 Neuron Cores. A 2-sized NCG will have 1x8GB and 2 Neuron Cores. The 8GB cache memory is shared between all 4 Neuron Cores of an Inferentia chip.

Before a model is deployed on ASIC hardware, it first has to be compiled for the said hardware. The Neuron compiler can be used to convert a regular TF SavedModel or PyTorch model into hardware-specific instruction set for Inferentia. Cortex currently supports TensorFlow and PyTorch compiled models.

### Compiling Models

By default, the neuron compiler will try to compile a model to use 1 Neuron core, but can be manually set to a different size (1, 2, 4, etc). To understand why setting a higher Neuron core count can improve performance, read [NeuronCore Pipeline notes](https://github.com/aws/aws-neuron-sdk/blob/master/docs/technotes/neuroncore-pipeline.md).

```python
# for TensorFlow SavedModel
import tensorflow.neuron as tfn
tfn.saved_model.compile(
    model_dir,
    compiled_model_dir,
    compiler_args=["--num-neuroncores", "1"]
)

# for PyTorch model
import torch_neuron, torch
model.eval()
model_neuron = torch.neuron.trace(
    model,
    example_inputs=[example_input],
    compiler_args=["--num-neuroncores", "1"]
)
model_neuron.save(compiled_model)
```

The current versions of `tensorflow-neuron` and `torch-neuron` are found in the [pre-installed packages list](predictors.md#for-asic-equipped-apis). To compile models of your own, these packages have to installed using the extra index URL for pip `--extra-index-url=https://pip.repos.neuron.amazonaws.com`.

See the [TensorFlow](https://github.com/aws/aws-neuron-sdk/blob/master/docs/tensorflow-neuron/tutorial-compile-infer.md#step-3-compile-on-compilation-instance) and the [PyTorch](https://github.com/aws/aws-neuron-sdk/blob/master/docs/pytorch-neuron/tutorial-compile-infer.md#step-3-compile-on-compilation-instance) guides on how to compile models to be used on ASIC hardware. There are 2 examples implemented with Cortex for both frameworks:

1. ResNet50 [example model](https://github.com/cortexlabs/cortex/tree/master/examples/tensorflow/image-classifier-resnet50) implemented for TensorFlow.
1. ResNet50 [example model](https://github.com/cortexlabs/cortex/tree/master/examples/pytorch/image-classifier-resnet50) implemented for PyTorch.

### Increasing Performance

A few things can be done to improve performance using compiled models on Cortex:

1. There's a minimum number of Neuron Cores for which a model can be compiled for. That number depends on the model's architecture. Generally, compiling a model for more cores than its required minimum helps at distributing the model's operators across multiple cores, which in turn can lead to lower latencies, but due to having to set [`compute:workers_per_replica`'s value](autoscaling.md#replica-parallelism) to a smaller value, the maximum throughput will be reduced. For higher throughput and higher latency, compile the models for as few Neuron Cores as possible using the `--num-neuroncores` compiler option and increase [`compute:workers_per_replica`'s value](autoscaling.md#replica-parallelism) to the maximum allowed.
1. Try to achieve a near [100% placement](https://github.com/aws/aws-neuron-sdk/blob/b28262e3072574c514a0d72ad3fe5ca48686d449/src/examples/tensorflow/keras_resnet50/pb2sm_compile.py#L59) of the model's graph onto the Neuron Cores. During the compilation phase, converted operators that can't execute on Neuron Cores will be compiled to execute on the machine's CPU and memory instead. If the model is not 100% compatible with the Neuron Cores' instruction set, then expect bits of the model to execute/reside on the machine's CPU/memory. Even if just a few percent of them reside on the host's, the maximum throughput capacity of the instance can get severly limited.
1. Use [`--static-weights` compiler option](https://github.com/aws/aws-neuron-sdk/blob/master/docs/technotes/performance-tuning.md#compiling-for-pipeline-optimization) where possible. This option tells the compiler to make it such that the whole model gets cached onto the Neuron Cores (NCG). This avoids a lot of back-and-forth between the machine's CPU/memory and the Inferentia chip(s).