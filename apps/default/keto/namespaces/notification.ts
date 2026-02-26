import { Namespace, Context } from "@ory/keto-namespace-types"

class profile implements Namespace {}

class notification_tenant implements Namespace {
  related: {
    owner: profile[]
    admin: profile[]
    operator: profile[]
    viewer: profile[]

    send_notification: profile[]
    release_notification: profile[]
    search_notifications: profile[]
    view_notification_status: profile[]
    update_notification_status: profile[]
    manage_template: profile[]
    view_template: profile[]
  }

  permits = {
    send_notification: (ctx: Context): boolean =>
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.send_notification.includes(ctx.subject),

    release_notification: (ctx: Context): boolean =>
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.release_notification.includes(ctx.subject),

    search_notifications: (ctx: Context): boolean =>
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.search_notifications.includes(ctx.subject),

    view_notification_status: (ctx: Context): boolean =>
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.view_notification_status.includes(ctx.subject),

    update_notification_status: (ctx: Context): boolean =>
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.update_notification_status.includes(ctx.subject),

    manage_template: (ctx: Context): boolean =>
      this.related.owner.includes(ctx.subject) ||
      this.related.admin.includes(ctx.subject) ||
      this.related.manage_template.includes(ctx.subject),

    view_template: (ctx: Context): boolean =>
      this.permits.manage_template(ctx) ||
      this.related.operator.includes(ctx.subject) ||
      this.related.viewer.includes(ctx.subject) ||
      this.related.view_template.includes(ctx.subject),
  }
}
