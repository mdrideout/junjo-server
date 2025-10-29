"""add_poller_state_table

Revision ID: cfacd8064837
Revises: dcabb962120e
Create Date: 2025-10-28 21:09:33.277658

"""
from typing import Sequence, Union

from alembic import op
import sqlalchemy as sa


# revision identifiers, used by Alembic.
revision: str = 'cfacd8064837'
down_revision: Union[str, Sequence[str], None] = 'dcabb962120e'
branch_labels: Union[str, Sequence[str], None] = None
depends_on: Union[str, Sequence[str], None] = None


def upgrade() -> None:
    """Create poller_state table."""
    op.create_table(
        'poller_state',
        sa.Column('id', sa.Integer(), nullable=False),
        sa.Column('last_key', sa.LargeBinary(), nullable=True),
        sa.PrimaryKeyConstraint('id'),
        sa.CheckConstraint('id = 1', name='single_row_check')
    )


def downgrade() -> None:
    """Drop poller_state table."""
    op.drop_table('poller_state')
