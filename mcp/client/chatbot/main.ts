/**
 * Responds to a message in Google Chat.
 *
 * @param {Object} event the event object from Google Workspace add-on
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function onMessage(event: any) {
  const message = event.chat.messagePayload.message;
  const user = event.chat.user;

  let name = '';
  if (message.space.type === 'DM') {
    name = 'You';
  } else {
    name = user.displayName || 'User';
  }

  const responseMessage = `${name} said "${message.text}"`;
  return {
    hostAppDataAction: {
      chatDataAction: {
        createMessageAction: {
          message: {
            text: responseMessage,
          },
        },
      },
    },
  };
}

/**
 * Responds to being added to a Google Chat space.
 *
 * @param {Object} event the event object from Google Workspace add-on
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function onAddedToSpace(event: any) {
  const space = event.chat.addedToSpacePayload.space;
  const user = event.chat.user;

  // If added through @mention a separate MESSAGE event is sent.
  let message = '';
  if (space.singleUserBotDm) {
    message = `Thank you for adding me to a direct message,
    ${user.displayName || 'User'}!`;
  } else {
    message = `Thank you for adding me to ${space.displayName || 'this space'}`;
  }

  return {
    hostAppDataAction: {
      chatDataAction: {
        createMessageAction: {
          message: {
            text: message,
          },
        },
      },
    },
  };
}

/**
 * Responds to being removed from a Google Chat space.
 *
 * @param {Object} event the event object from Google Workspace add-on
 */
// eslint-disable-next-line @typescript-eslint/no-unused-vars
function onRemovedFromSpace(event: any) {
  const space = event.chat.removedFromSpacePayload.space;
  console.info(`Chat app removed from ${space.name || 'this chat'}`);
}
