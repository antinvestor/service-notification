# Service-specific configuration
SERVICE_NAME := notification
APP_DIRS     := apps/default apps/ussd apps/integrations/africastalking apps/integrations/emailsmtp apps/integrations/smpp

# Bootstrap: download shared Makefile.common if missing
ifeq (,$(wildcard .tmp/Makefile.common))
  $(shell mkdir -p .tmp && curl -sSfL https://raw.githubusercontent.com/antinvestor/common/main/Makefile.common -o .tmp/Makefile.common)
endif

include .tmp/Makefile.common
