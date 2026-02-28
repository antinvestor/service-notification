import { Namespace, Context } from "@ory/keto-namespace-types"

class profile_user implements Namespace {}

class tenancy_access implements Namespace {
  related: {
    member: (profile_user | tenancy_access)[]
    service: profile_user[]
  }
}

class service_notifications implements Namespace {
  related: {
    owner: profile_user[]
    admin: profile_user[]
    operator: profile_user[]
    viewer: profile_user[]
    member: profile_user[]
    service: (profile_user | tenancy_access)[]

    // Direct permission grants (accept service_notifications subject sets for service role bridging)
    send_notification: (profile_user | service_notifications)[]
    release_notification: (profile_user | service_notifications)[]
    search_notifications: (profile_user | service_notifications)[]
    view_notification_status: (profile_user | service_notifications)[]
    update_notification_status: (profile_user | service_notifications)[]
    manage_template: (profile_user | service_notifications)[]
    view_template: (profile_user | service_notifications)[]
  }

  permits = {
    send_notification: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.send_notification.includes(ctx.subject),

    release_notification: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.release_notification.includes(ctx.subject),

    search_notifications: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.member.includes(ctx.subject) ||
      this.related.search_notifications.includes(ctx.subject),

    view_notification_status: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.member.includes(ctx.subject) ||
      this.related.view_notification_status.includes(ctx.subject),

    update_notification_status: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.update_notification_status.includes(ctx.subject),

    manage_template: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.manage_template.includes(ctx.subject),

    view_template: (ctx: Context): boolean =>
      this.related.service.includes(ctx.subject) ||
      this.permits.manage_template(ctx) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.view_template.includes(ctx.subject),
  }
}
