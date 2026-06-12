terraform {
  required_version = ">= 1.6.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.region
}

resource "aws_vpc" "this" {
  cidr_block           = "10.80.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name = "${var.name}-vpc"
  }
}

resource "aws_subnet" "validators" {
  count                   = var.validator_count
  vpc_id                  = aws_vpc.this.id
  cidr_block              = cidrsubnet(aws_vpc.this.cidr_block, 8, count.index)
  map_public_ip_on_launch = true

  tags = {
    Name       = "${var.name}-validator-${count.index + 1}"
    VexoRole   = "validator"
    VexoIndex  = tostring(count.index + 1)
  }
}

resource "aws_internet_gateway" "this" {
  vpc_id = aws_vpc.this.id

  tags = {
    Name = "${var.name}-igw"
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.this.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.this.id
  }

  tags = {
    Name = "${var.name}-public"
  }
}

resource "aws_route_table_association" "validators" {
  count          = var.validator_count
  subnet_id      = aws_subnet.validators[count.index].id
  route_table_id = aws_route_table.public.id
}

resource "aws_security_group" "validator" {
  name        = "${var.name}-validator"
  description = "Vexo validator network access"
  vpc_id      = aws_vpc.this.id

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "${var.name}-validator"
  }
}

resource "aws_vpc_security_group_ingress_rule" "ssh" {
  for_each          = toset(var.ssh_cidr_blocks)
  security_group_id = aws_security_group.validator.id
  cidr_ipv4         = each.value
  from_port         = 22
  ip_protocol       = "tcp"
  to_port           = 22
}

resource "aws_vpc_security_group_ingress_rule" "p2p" {
  for_each          = toset(var.p2p_cidr_blocks)
  security_group_id = aws_security_group.validator.id
  cidr_ipv4         = each.value
  from_port         = 26656
  ip_protocol       = "tcp"
  to_port           = 26656
}

resource "aws_vpc_security_group_ingress_rule" "rpc" {
  for_each          = toset(var.rpc_cidr_blocks)
  security_group_id = aws_security_group.validator.id
  cidr_ipv4         = each.value
  from_port         = 26657
  ip_protocol       = "tcp"
  to_port           = 26657
}

resource "aws_instance" "validator" {
  count                  = var.validator_count
  ami                    = var.ami_id
  instance_type          = var.instance_type
  subnet_id              = aws_subnet.validators[count.index].id
  vpc_security_group_ids = [aws_security_group.validator.id]
  key_name               = var.key_name

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
  }

  root_block_device {
    encrypted   = true
    volume_size = 100
    volume_type = "gp3"
  }

  tags = {
    Name      = "${var.name}-validator-${count.index + 1}"
    VexoRole  = "validator"
    VexoIndex = tostring(count.index + 1)
  }
}

