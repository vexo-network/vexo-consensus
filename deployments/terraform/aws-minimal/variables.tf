variable "name" {
  description = "Deployment name prefix."
  type        = string
  default     = "vexo"
}

variable "region" {
  description = "AWS region."
  type        = string
  default     = "us-east-1"
}

variable "validator_count" {
  description = "Number of validator hosts."
  type        = number
  default     = 4

  validation {
    condition     = var.validator_count >= 1 && var.validator_count <= 128
    error_message = "validator_count must be between 1 and 128."
  }
}

variable "instance_type" {
  description = "EC2 instance type."
  type        = string
  default     = "c7i.large"
}

variable "ami_id" {
  description = "AMI ID with your hardened base image."
  type        = string
}

variable "ssh_cidr_blocks" {
  description = "CIDR blocks allowed to SSH to validator hosts."
  type        = list(string)
  default     = []
}

variable "p2p_cidr_blocks" {
  description = "CIDR blocks allowed to reach Vexo P2P."
  type        = list(string)
  default     = ["0.0.0.0/0"]
}

variable "rpc_cidr_blocks" {
  description = "CIDR blocks allowed to reach Vexo RPC. Keep private unless intentionally exposing public RPC."
  type        = list(string)
  default     = []
}

variable "key_name" {
  description = "Optional EC2 key pair name."
  type        = string
  default     = null
}

