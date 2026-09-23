import landing from './landing'
import common from './common'
import dashboard from './dashboard'
import channelMonitorV2 from './channelMonitorV2'
import batchImage from './batchImage'
import admin from './admin'
import misc from './misc'
import accountIPChannelState from './accountIPChannelState'
import accountModelMismatch from './accountModelMismatch'
import accountTicketDefaults from './accountTicketDefaults'
import groupStatus from './groupStatus'

export default {
  ...landing,
  ...common,
  ...dashboard,
  ...channelMonitorV2,
  ...batchImage,
  admin,
  ...misc,
  ...accountIPChannelState,
  ...accountModelMismatch,
  ...accountTicketDefaults,
  ...groupStatus,
}
